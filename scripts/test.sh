#!/bin/bash
# Skillshare Test Runner
# Usage: ./test.sh [options]
#
# Options:
#   (no args)     Run all tests
#   -u, --unit    Run only unit tests
#   -i, --int     Run only integration tests
#   -c, --cover   Run with coverage report
#   -v, --verbose Verbose output (default)
#   -q, --quiet   Quiet output
#   -h, --help    Show this help

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Default options
VERBOSE="-v"
RUN_UNIT=true
RUN_INT=true
COVERAGE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--unit)
            RUN_UNIT=true
            RUN_INT=false
            shift
            ;;
        -i|--int|--integration)
            RUN_UNIT=false
            RUN_INT=true
            shift
            ;;
        -c|--cover|--coverage)
            COVERAGE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -q|--quiet)
            VERBOSE=""
            shift
            ;;
        -h|--help)
            echo "Usage: ./test.sh [options]"
            echo ""
            echo "Options:"
            echo "  (no args)     Run all tests"
            echo "  -u, --unit    Run only unit tests"
            echo "  -i, --int     Run only integration tests"
            echo "  -c, --cover   Run with coverage report"
            echo "  -v, --verbose Verbose output (default)"
            echo "  -q, --quiet   Quiet output"
            echo "  -h, --help    Show this help"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Get project root
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

# Build binary for integration tests
if [[ "$RUN_INT" == true ]]; then
    echo -e "${YELLOW}Building binary...${NC}"
    make build
    export SKILLSHARE_TEST_BINARY="$PROJECT_ROOT/bin/skillshare"
    echo -e "${GREEN}✓ Binary built: bin/skillshare${NC}"
    echo ""
fi

# Prepare coverage flag
COVER_FLAG=""
if [[ "$COVERAGE" == true ]]; then
    COVER_FLAG="-cover"
fi

# Run unit tests
if [[ "$RUN_UNIT" == true ]]; then
    echo -e "${YELLOW}Running unit tests...${NC}"
    go test $VERBOSE $COVER_FLAG ./internal/...
    echo ""
fi

# Run integration tests
if [[ "$RUN_INT" == true ]]; then
    echo -e "${YELLOW}Running integration tests...${NC}"
    go test $VERBOSE $COVER_FLAG ./tests/integration/...
    echo ""
fi

echo -e "${GREEN}✓ All tests passed!${NC}"
