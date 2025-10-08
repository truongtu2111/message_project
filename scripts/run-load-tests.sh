#!/bin/bash

# Load Testing Automation Script for Insider Messaging
# This script runs all k6 load tests and generates reports

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="test/results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v k6 &> /dev/null; then
        log_error "k6 is not installed. Please install k6 first."
        log_info "Visit: https://k6.io/docs/getting-started/installation/"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_error "curl is not installed."
        exit 1
    fi
    
    log_success "All dependencies are available"
}

check_service_health() {
    log_info "Checking service health at $BASE_URL..."
    
    if curl -f -s "$BASE_URL/health" > /dev/null; then
        log_success "Service is healthy and ready for testing"
    else
        log_error "Service is not responding at $BASE_URL/health"
        log_info "Please ensure the service is running before running load tests"
        exit 1
    fi
}

create_results_directory() {
    mkdir -p "$RESULTS_DIR"
    log_info "Results will be saved to: $RESULTS_DIR"
}

run_load_test() {
    local test_name="$1"
    local test_file="$2"
    local output_file="$RESULTS_DIR/${test_name}_${TIMESTAMP}.json"
    
    log_info "Running $test_name..."
    log_info "Test file: $test_file"
    log_info "Output file: $output_file"
    
    if BASE_URL="$BASE_URL" k6 run --out json="$output_file" "$test_file"; then
        log_success "$test_name completed successfully"
        return 0
    else
        log_error "$test_name failed"
        return 1
    fi
}

generate_summary_report() {
    local report_file="$RESULTS_DIR/load_test_summary_${TIMESTAMP}.txt"
    
    log_info "Generating summary report: $report_file"
    
    cat > "$report_file" << EOF
Load Testing Summary Report
Generated: $(date)
Target URL: $BASE_URL
Test Session: $TIMESTAMP

===========================================

EOF

    # Add individual test summaries if result files exist
    for result_file in "$RESULTS_DIR"/*_"$TIMESTAMP".json; do
        if [[ -f "$result_file" ]]; then
            test_name=$(basename "$result_file" | sed "s/_${TIMESTAMP}.json//")
            echo "Test: $test_name" >> "$report_file"
            echo "Result File: $result_file" >> "$report_file"
            echo "---" >> "$report_file"
        fi
    done
    
    cat >> "$report_file" << EOF

===========================================

To analyze detailed results, use:
- k6 cloud results <result-file.json> (if using k6 cloud)
- Custom analysis tools with the JSON output files

For more information, see: test/load/README.md
EOF

    log_success "Summary report generated: $report_file"
}

cleanup_old_results() {
    log_info "Cleaning up old test results (keeping last 10)..."
    
    # Keep only the 10 most recent result files
    find "$RESULTS_DIR" -name "*.json" -type f | sort -r | tail -n +11 | xargs -r rm -f
    find "$RESULTS_DIR" -name "*.txt" -type f | sort -r | tail -n +11 | xargs -r rm -f
    
    log_success "Cleanup completed"
}

print_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
    -u, --url URL       Base URL for the service (default: http://localhost:8080)
    -t, --test TEST     Run specific test only (load|stress|spike|all)
    -h, --help          Show this help message
    --no-cleanup        Skip cleanup of old results
    --skip-health       Skip health check (use with caution)

Examples:
    $0                                  # Run all tests with default settings
    $0 -u http://staging.example.com    # Run against staging environment
    $0 -t load                          # Run only load test
    $0 -t stress --no-cleanup           # Run stress test without cleanup

Test Types:
    load    - Normal load testing (realistic user patterns)
    stress  - Stress testing (high load, identify breaking points)
    spike   - Spike testing (sudden traffic spikes)
    all     - Run all tests in sequence (default)
EOF
}

main() {
    local test_type="all"
    local skip_cleanup=false
    local skip_health=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -u|--url)
                BASE_URL="$2"
                shift 2
                ;;
            -t|--test)
                test_type="$2"
                shift 2
                ;;
            --no-cleanup)
                skip_cleanup=true
                shift
                ;;
            --skip-health)
                skip_health=true
                shift
                ;;
            -h|--help)
                print_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                print_usage
                exit 1
                ;;
        esac
    done
    
    log_info "Starting load testing session"
    log_info "Target URL: $BASE_URL"
    log_info "Test type: $test_type"
    
    # Pre-flight checks
    check_dependencies
    
    if [[ "$skip_health" != true ]]; then
        check_service_health
    fi
    
    create_results_directory
    
    if [[ "$skip_cleanup" != true ]]; then
        cleanup_old_results
    fi
    
    # Run tests based on type
    local failed_tests=0
    
    case $test_type in
        load)
            run_load_test "load-test" "test/load/k6-load-test.js" || ((failed_tests++))
            ;;
        stress)
            run_load_test "stress-test" "test/load/k6-stress-test.js" || ((failed_tests++))
            ;;
        spike)
            run_load_test "spike-test" "test/load/k6-spike-test.js" || ((failed_tests++))
            ;;
        all)
            log_info "Running all load tests in sequence..."
            run_load_test "load-test" "test/load/k6-load-test.js" || ((failed_tests++))
            sleep 5  # Brief pause between tests
            run_load_test "stress-test" "test/load/k6-stress-test.js" || ((failed_tests++))
            sleep 5  # Brief pause between tests
            run_load_test "spike-test" "test/load/k6-spike-test.js" || ((failed_tests++))
            ;;
        *)
            log_error "Invalid test type: $test_type"
            log_info "Valid types: load, stress, spike, all"
            exit 1
            ;;
    esac
    
    # Generate summary report
    generate_summary_report
    
    # Final status
    if [[ $failed_tests -eq 0 ]]; then
        log_success "All load tests completed successfully!"
        log_info "Check the results in: $RESULTS_DIR"
    else
        log_warning "$failed_tests test(s) failed"
        log_info "Check the results and logs for details"
        exit 1
    fi
}

# Run main function with all arguments
main "$@"