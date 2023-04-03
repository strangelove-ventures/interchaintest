// Package label contains types to manage labels for tests.
// Labels are treated as named values that are present or absent on a particular testutil.
//
// The labels are reported through the testutil reporter, in the JSON output.
// There is currently no support for using labels to filter which tests to run,
// but it should be trivial to postprocess the testutil report using labels.
package label
