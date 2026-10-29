PYTHON ?= python3

.PHONY: bootstrap baseline test-verification test-agent-contract

bootstrap:
	$(PYTHON) scripts/verification/baseline.py --bootstrap-only

baseline:
	$(PYTHON) scripts/verification/baseline.py

test-verification:
	$(PYTHON) -m unittest discover -s scripts/verification -p 'test_baseline.py' -v

test-agent-contract:
	$(PYTHON) -m unittest discover -s scripts/verification -p 'test_contract.py' -v
