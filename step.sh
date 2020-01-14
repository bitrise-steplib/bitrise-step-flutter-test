#!/usr/bin/env bash
# fail if any commands fails
# debug log

export PATH="$PATH":"/usr/local/flutter/.pub-cache/bin"
export PATH="$PATH":"/usr/local/flutter/bin/cache/dart-sdk/bin"

flutter pub get
flutter pub global activate junitreport

test_run_dir="$BITRISE_TEST_RESULT_DIR/result_dir_1"
mkdir "$test_run_dir"

flutter test --machine > test_results.jsonl
tojunit -i test_results.jsonl -o $test_run_dir/TEST-report.xml
echo '{"test-name":"tests-batch-1"}' >> "$test_run_dir/test-info.json"


