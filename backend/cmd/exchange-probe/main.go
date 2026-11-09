package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/zeroclap/gembo/internal/config"
	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	binanceex "github.com/zeroclap/gembo/internal/exchanges/binance"
	bybitex "github.com/zeroclap/gembo/internal/exchanges/bybit"
	gateex "github.com/zeroclap/gembo/internal/exchanges/gate"
	okxex "github.com/zeroclap/gembo/internal/exchanges/okx"
)

type checkResult struct {
	name   string
	ok     bool
	detail string
}

func main() {
	_ = godotenv.Load()

	var (
		cfgPath       string
		exchangeList  string
		symbolList    string
		timeout       time.Duration
		marketSample  int
		fundingSample int
		withWS        bool
		tickerSamples int
		withPrivate   bool
		printEgress   bool
		egressURL     string
	)

	flag.StringVar(&cfgPath, "config", os.Getenv("CONFIG_PATH"), "path to config file")
	flag.StringVar(&exchangeList, "exchanges", "bybit,okx,binance,gate", "comma-separated exchange IDs")
	flag.StringVar(&symbolList, "symbols", "BTC/USDT:USDT", "comma-separated symbols")
	flag.DurationVar(&timeout, "timeout", 15*time.Second, "per-check timeout")
	flag.IntVar(&marketSample, "market-sample", 5, "number of markets to print")
	flag.IntVar(&fundingSample, "funding-sample", 5, "number of funding rows to read")
	flag.BoolVar(&withWS, "ws", false, "run one websocket ticker check")
	flag.IntVar(&tickerSamples, "ticker-samples", 1, "with -ws: bid/ask quotes to collect; reports source->receive age p50/p95/max and fails if source time is missing")
	flag.BoolVar(&withPrivate, "private", false, "run authenticated checks (balances/positions)")
	flag.BoolVar(&printEgress, "print-egress", false, "print the public-IP egress country and fail if it is US (non-US CI egress, issue #4)")
	flag.StringVar(&egressURL, "egress-url", "https://ifconfig.co/country-iso", "URL returning the 2-letter egress country code as plain text")
	flag.Parse()

	if printEgress {
		country, err := fetchEgressCountry(context.Background(), egressURL, timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "egress country check: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("egress_country=%s\n", country)
		if strings.EqualFold(country, "US") {
			fmt.Fprintln(os.Stderr, "egress country is US; Bybit/Binance require non-US CI egress")
			os.Exit(1)
		}
	}

	if cfgPath == "" {
		cfgPath = "configs/config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	wantedExchanges := parseCSV(exchangeList)
	if len(wantedExchanges) == 0 {
		fmt.Fprintln(os.Stderr, "no exchanges requested")
		os.Exit(1)
	}

	symbols := parseCSV(symbolList)
	if len(symbols) == 0 {
		fmt.Fprintln(os.Stderr, "no symbols requested")
		os.Exit(1)
	}

	byID := make(map[string]config.ExchangeCfg, len(cfg.Exchanges))
	for _, ex := range cfg.Exchanges {
		byID[ex.ID] = ex
	}

	fmt.Printf("exchange probe: exchanges=%s symbols=%s timeout=%s ws=%v private=%v\n\n",
		strings.Join(wantedExchanges, ","), strings.Join(symbols, ","), timeout, withWS, withPrivate)

	failed := false
	for _, id := range wantedExchanges {
		ex, err := newExchangeClient(id, logger, byID[id], withPrivate)
		if err != nil {
			failed = true
			fmt.Printf("[%s] init: FAIL (%v)\n\n", id, err)
			continue
		}

		results := runChecks(ex, id, symbols, timeout, marketSample, fundingSample, tickerSamples, withWS, withPrivate)
		printResults(id, results)
		if hasFailure(results) {
			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
}

func newExchangeClient(id string, logger *slog.Logger, cfg config.ExchangeCfg, withPrivate bool) (base.Exchange, error) {
	switch id {
	case "bybit":
		opts := []bybitex.Option{}
		if withPrivate {
			opts = append(opts,
				bybitex.WithAPIKey(cfg.APIKey),
				bybitex.WithSecret(cfg.Secret),
			)
		}
		return bybitex.New(logger, opts...), nil
	case "okx":
		opts := []okxex.Option{}
		if withPrivate {
			opts = append(opts,
				okxex.WithAPIKey(cfg.APIKey),
				okxex.WithSecret(cfg.Secret),
				okxex.WithPassphrase(cfg.Passphrase),
			)
		}
		return okxex.New(logger, opts...), nil
	case "binance":
		opts := []binanceex.Option{}
		if withPrivate {
			opts = append(opts,
				binanceex.WithAPIKey(cfg.APIKey),
				binanceex.WithSecret(cfg.Secret),
			)
		}
		return binanceex.New(logger, opts...), nil
	case "gate":
		if withPrivate {
			return nil, fmt.Errorf("gate private checks are not implemented in this repo")
		}
		return gateex.New(logger), nil
	default:
		return nil, fmt.Errorf("unsupported exchange %q", id)
	}
}

func runChecks(
	ex base.Exchange,
	id string,
	symbols []string,
	timeout time.Duration,
	marketSample int,
	fundingSample int,
	tickerSamples int,
	withWS bool,
	withPrivate bool,
) []checkResult {
	results := make([]checkResult, 0, 5)

	results = append(results, checkMarkets(ex, symbols, timeout, marketSample))
	results = append(results, checkFunding(ex, timeout, fundingSample))
	if withWS {
		results = append(results, checkTickers(ex, symbols, timeout, tickerSamples))
	}
	if withPrivate {
		if px, ok := ex.(base.PrivateExchange); ok {
			results = append(results, checkPrivate(px, id, symbols, timeout)...)
		} else {
			results = append(results, checkResult{
				name:   "private",
				ok:     false,
				detail: "exchange does not implement private interface",
			})
		}
	}

	return results
}

func checkMarkets(ex base.Exchange, symbols []string, timeout time.Duration, sample int) checkResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	markets, err := ex.Markets(ctx)
	if err != nil {
		return checkResult{name: "markets", ok: false, detail: err.Error()}
	}
	if len(markets) == 0 {
		return checkResult{name: "markets", ok: false, detail: "empty response"}
	}

	available := make(map[string]struct{}, len(markets))
	for _, m := range markets {
		available[m.Symbol] = struct{}{}
	}

	missing := make([]string, 0)
	for _, s := range symbols {
		if _, ok := available[s]; !ok {
			missing = append(missing, s)
		}
	}

	shown := sample
	if shown <= 0 || shown > len(markets) {
		shown = len(markets)
	}
	parts := make([]string, 0, shown)
	for i := 0; i < shown; i++ {
		parts = append(parts, markets[i].Symbol)
	}

	detail := fmt.Sprintf("count=%d sample=%s", len(markets), strings.Join(parts, ","))
	if len(missing) > 0 {
		detail += fmt.Sprintf(" missing_symbols=%s", strings.Join(missing, ","))
	}

	return checkResult{name: "markets", ok: true, detail: detail}
}

func checkFunding(ex base.Exchange, timeout time.Duration, sample int) checkResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ch, err := ex.FundingRates(ctx)
	if err != nil {
		return checkResult{name: "funding", ok: false, detail: err.Error()}
	}

	if sample <= 0 {
		sample = 1
	}

	rows := make([]domain.FundingRate, 0, sample)
	for len(rows) < sample {
		select {
		case <-ctx.Done():
			if len(rows) == 0 {
				return checkResult{name: "funding", ok: false, detail: "no rows before timeout"}
			}
			return checkResult{name: "funding", ok: true, detail: summarizeFunding(rows)}
		case fr, ok := <-ch:
			if !ok {
				if len(rows) == 0 {
					return checkResult{name: "funding", ok: false, detail: "channel closed without data"}
				}
				return checkResult{name: "funding", ok: true, detail: summarizeFunding(rows)}
			}
			rows = append(rows, fr)
		}
	}

	return checkResult{name: "funding", ok: true, detail: summarizeFunding(rows)}
}

func checkTickers(ex base.Exchange, symbols []string, timeout time.Duration, samples int) checkResult {
	if samples <= 0 {
		samples = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ch, err := ex.Tickers(ctx, symbols)
	if err != nil {
		return checkResult{name: "tickers", ok: false, detail: err.Error()}
	}

	var (
		received      int
		last          domain.Ticker
		ages          []time.Duration
		missingSource int
	)
	for len(ages)+missingSource < samples {
		select {
		case <-ctx.Done():
			return tickerResult(received, last, ages, missingSource, "no ticker before timeout")
		case t, ok := <-ch:
			if !ok {
				return tickerResult(received, last, ages, missingSource, "channel closed without ticker")
			}
			received++
			last = t
			// Partial updates without a bid/ask are not arbitrage quotes.
			if !t.Bid.IsPositive() || !t.Ask.IsPositive() {
				continue
			}
			if t.SourceTime.IsZero() {
				missingSource++
				continue
			}
			ages = append(ages, t.ReceivedAt.Sub(t.SourceTime))
		}
	}
	return tickerResult(received, last, ages, missingSource, "")
}

// tickerResult builds the tickers check result. Any quote without an exchange
// timestamp fails the check: such quotes are rejected by the arbitrage worker.
func tickerResult(received int, last domain.Ticker, ages []time.Duration, missingSource int, noData string) checkResult {
	if received == 0 {
		return checkResult{name: "tickers", ok: false, detail: noData}
	}
	detail := fmt.Sprintf("received=%d symbol=%s bid=%s ask=%s price=%s", received, last.Symbol, last.Bid, last.Ask, last.Price)
	if len(ages) > 0 {
		p50, p95, maxAge := agePercentiles(ages)
		detail += fmt.Sprintf(" source_age_samples=%d p50=%s p95=%s max=%s", len(ages), p50, p95, maxAge)
	}
	if missingSource > 0 {
		return checkResult{
			name:   "tickers",
			ok:     false,
			detail: fmt.Sprintf("%d quote(s) without source time (rejected by arbitrage); %s", missingSource, detail),
		}
	}
	return checkResult{name: "tickers", ok: true, detail: detail}
}

// agePercentiles returns the nearest-rank p50 and p95 and the max of ages.
// ages must be non-empty.
func agePercentiles(ages []time.Duration) (p50, p95, maxAge time.Duration) {
	sorted := append([]time.Duration(nil), ages...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := func(p float64) time.Duration {
		idx := int(math.Ceil(p*float64(len(sorted)))) - 1
		if idx < 0 {
			idx = 0
		}
		return sorted[idx]
	}
	return rank(0.50), rank(0.95), sorted[len(sorted)-1]
}

func checkPrivate(px base.PrivateExchange, id string, symbols []string, timeout time.Duration) []checkResult {
	results := make([]checkResult, 0, len(symbols)+2)

	if !px.HasCredentials() {
		results = append(results, checkResult{
			name:   "private.credentials",
			ok:     false,
			detail: "missing API credentials in config/env",
		})
		return results
	}
	results = append(results, checkResult{name: "private.credentials", ok: true, detail: "credentials provided"})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	balances, err := px.FetchBalance(ctx)
	cancel()
	if err != nil {
		results = append(results, checkResult{name: "private.balance", ok: false, detail: err.Error()})
	} else {
		results = append(results, checkResult{name: "private.balance", ok: true, detail: summarizeBalances(balances)})
	}

	for _, symbol := range symbols {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		pos, perr := px.FetchPosition(ctx, symbol)
		cancel()
		if perr != nil {
			results = append(results, checkResult{
				name:   fmt.Sprintf("private.position[%s]", symbol),
				ok:     false,
				detail: perr.Error(),
			})
			continue
		}
		if pos == nil {
			results = append(results, checkResult{
				name:   fmt.Sprintf("private.position[%s]", symbol),
				ok:     true,
				detail: "no open position",
			})
			continue
		}
		results = append(results, checkResult{
			name: fmt.Sprintf("private.position[%s]", symbol),
			ok:   true,
			detail: fmt.Sprintf(
				"exchange=%s side=%s contracts=%s entry=%s mark=%s uPnL=%s",
				id,
				pos.Side,
				pos.Contracts,
				pos.EntryPrice,
				pos.MarkPrice,
				pos.UnrealizedPnL,
			),
		})
	}

	return results
}

func summarizeFunding(rows []domain.FundingRate) string {
	symbols := make([]string, 0, len(rows))
	for _, r := range rows {
		symbols = append(symbols, r.Symbol)
	}
	sort.Strings(symbols)
	return fmt.Sprintf("rows=%d symbols=%s", len(rows), strings.Join(symbols, ","))
}

func summarizeBalances(balances []base.Balance) string {
	if len(balances) == 0 {
		return "rows=0"
	}
	parts := make([]string, 0, len(balances))
	for _, b := range balances {
		if !b.Total.IsPositive() {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", b.Currency, b.Total))
		if len(parts) >= 5 {
			break
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("rows=%d positive_totals=0", len(balances))
	}
	return fmt.Sprintf("rows=%d sample=%s", len(balances), strings.Join(parts, ","))
}

func hasFailure(results []checkResult) bool {
	for _, r := range results {
		if !r.ok {
			return true
		}
	}
	return false
}

func printResults(exchangeID string, results []checkResult) {
	fmt.Printf("[%s]\n", exchangeID)
	for _, r := range results {
		status := "OK"
		if !r.ok {
			status = "FAIL"
		}
		fmt.Printf("  - %-24s %s  %s\n", r.name, status, r.detail)
	}
	fmt.Println()
}

func fetchEgressCountry(ctx context.Context, url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build egress country request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch egress country: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch egress country: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", fmt.Errorf("read egress country response: %w", err)
	}

	return strings.ToUpper(strings.TrimSpace(string(body))), nil
}

func parseCSV(v string) []string {
	if v == "" {
		return nil
	}
	out := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range strings.Split(v, ",") {
		s := strings.TrimSpace(item)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
