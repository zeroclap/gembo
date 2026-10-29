PYTHON ?= python3

.PHONY: bootstrap baseline test-verification

bootstrap:
	$(PYTHON) scripts/verification/baseline.py --bootstrap-only

baseline:
	$(PYTHON) scripts/verification/baseline.py

test-verification:
	$(PYTHON) -m unittest discover -s scripts/verification -p 'test_*.py' -v
