#!/usr/bin/env bash
# fail if any commands fails
set -e
# debug log

export PATH="$PATH":"/usr/local/flutter/.pub-cache/bin"
export PATH="$PATH":"/usr/local/flutter/bin/cache/dart-sdk/bin"

flutter pub get
flutter pub global activate junitreport

test_run_dir="$BITRISE_TEST_RESULT_DIR/flutter_tests_results"
mkdir "$test_run_dir"

echo "Running flutter test --machine $additional_params"
flutter test --machine $additional_params > test_results.jsonl
tojunit -i test_results.jsonl -o $test_run_dir/TEST-report.xml
echo "{\"test-name\":\"$test_run_name\"}" >> "$test_run_dir/test-info.json"
