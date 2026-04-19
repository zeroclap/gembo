package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/joho/godotenv"

	"github.com/zeroclap/gembo/internal/config"
)

func main() {
	_ = godotenv.Load()

	var cfgPath string
	var symbol string
	flag.StringVar(&cfgPath, "config", envOrDefault("CONFIG_PATH", "configs/config.yaml"), "path to config file")
	flag.StringVar(&symbol, "symbol", "ENJ/USDT:USDT", "market symbol")
	flag.Parse()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	normalized := normalizeSymbol(symbol)

	bybitCfg, ok := exchangeCfg(cfg, "bybit")
	if !ok {
		fmt.Fprintln(os.Stderr, "bybit config not found")
		os.Exit(1)
	}
	okxCfg, ok := exchangeCfg(cfg, "okx")
	if !ok {
		fmt.Fprintln(os.Stderr, "okx config not found")
		os.Exit(1)
	}

	bybit := ccxtpro.NewBybit(map[string]interface{}{
		"apiKey": bybitCfg.APIKey,
		"secret": bybitCfg.Secret,
		"options": map[string]any{
			"defaultType": "swap",
		},
	})
	defer bybit.Close()

	okx := ccxtpro.NewOkx(map[string]interface{}{
		"apiKey":   okxCfg.APIKey,
		"secret":   okxCfg.Secret,
		"password": okxCfg.Passphrase,
		"options": map[string]any{
			"defaultType": "swap",
		},
	})
	defer okx.Close()

	fetchAndPrintBybit(bybit, normalized)
	fetchAndPrintOKX(okx, normalized)
}

func fetchAndPrintBybit(client *ccxtpro.Bybit, symbol string) {
	pos, err := client.FetchPosition(symbol, ccxt.WithFetchPositionParams(map[string]any{
		"acknowledged": true,
	}))
	if err != nil {
		printError("bybit", err)
		return
	}
	printJSON("bybit", pos)
}

func fetchAndPrintOKX(client *ccxtpro.Okx, symbol string) {
	pos, err := client.FetchPosition(symbol)
	if err != nil {
		printError("okx", err)
		return
	}
	printJSON("okx", pos)
}

func printJSON(exchangeID string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("=== %s ===\nmarshal error: %v\n", exchangeID, err)
		return
	}
	fmt.Printf("=== %s ===\n%s\n", exchangeID, string(b))
}

func printError(exchangeID string, err error) {
	fmt.Printf("=== %s ===\nerror: %v\n", exchangeID, err)
}

func exchangeCfg(cfg *config.Config, id string) (config.ExchangeCfg, bool) {
	for _, ex := range cfg.Exchanges {
		if ex.ID == id {
			return ex, true
		}
	}
	return config.ExchangeCfg{}, false
}

func normalizeSymbol(symbol string) string {
	s := strings.TrimSpace(symbol)
	if strings.Contains(s, ":") && strings.Contains(s, "/") {
		parts := strings.Split(s, ":")
		if len(parts) == 2 && strings.Contains(parts[1], "/") {
			left := parts[0]
			right := parts[1]
			rp := strings.Split(right, "/")
			if len(rp) == 2 {
				return rp[0] + "/" + left + ":" + rp[1]
			}
		}
	}
	return s
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
