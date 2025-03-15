package main

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

func ZeroRefreshCLI() RefreshCLI {
	return RefreshCLI{Path: "TEST", TrackedFilesAction: "TEST", Propagate: false, ExemptionsSrc: "TEST", Pull: false}
}

func TestNonInputList(t *testing.T) {
	new_cli := DefaultRefreshCLI()
	non_input_options := new_cli.ToNonInputOptions()
	invalid_non_input_option := new_cli.FromNonInputList(non_input_options)
	if invalid_non_input_option != nil {
		t.Fatalf(
			".ToNonInputOptions unaligned with NonInputOptions accepted by FromNonInputList: %q",
			invalid_non_input_option,
		)
	}

	// .FromNonInputList should fail -- invalid options
	cli_invalid := DefaultRefreshCLI()
	valid_inputs := cli_invalid.ToNonInputOptions()
	invalid_inputs := append(valid_inputs, "--invalid")
	expect_invalid := cli_invalid.FromNonInputList(invalid_inputs)
	if expect_invalid == nil {
		t.Fatal("Invalid option put into options list but did not trigger error")
	}
}

func TestRefreshCLIConfigMapping(t *testing.T) {
	delta_opt := "restore"
	invalid_delta_opt := "fake_option"
	_, valid_was_parsed := deltaParse(delta_opt)
	if valid_was_parsed != nil {
		t.Fatalf("Valid option raised error %q", valid_was_parsed)
	}
	invalid_out, invalid_was_prased := deltaParse(invalid_delta_opt)
	if invalid_was_prased == nil {
		t.Fatalf("Invalid option %q incorrectly was mapped to %q", invalid_delta_opt, invalid_out)
	}
}

func TestFromInteractiveMap(t *testing.T) {
	valid_delta_cli := DefaultRefreshCLI()
	valid_delta_map := map[string]string{"--delta": "stash"}
	valid_delta_return := valid_delta_cli.FromInteractiveMap(valid_delta_map)
	if valid_delta_return != nil {
		t.Fatalf("Error when applying valid interactive map to RefreshCLI %v", valid_delta_return)
	} else if DefaultRefreshCLI() == valid_delta_cli {
		t.Fatalf("RefreshCLI unexpectedly has default values: %v", valid_delta_cli)
	}
	invalid_delta_cli := DefaultRefreshCLI()
	invalid_delta_map := map[string]string{"-d": "fake_delta_option"}
	invalid_delta_return := invalid_delta_cli.FromInteractiveMap(invalid_delta_map)
	if invalid_delta_return == nil {
		t.Fatalf("Error when applying interactive map to RefreshCLI: invalid success for map %v", invalid_delta_map)
	}

	valid_exempt_cli := DefaultRefreshCLI()
	valid_exempt_map := map[string]string{"--exemptions": "git-ignore"}
	valid_exempt_return := valid_exempt_cli.FromInteractiveMap(valid_exempt_map)
	if valid_exempt_return != nil {
		t.Fatalf("Error when applying valid interactive map to RefreshCLI %v", valid_exempt_return)
	} else if DefaultRefreshCLI() == valid_exempt_cli {
		t.Fatalf("RefreshCLI had unexpected value: %v", valid_exempt_cli)
	}
	invalid_exempt_cli := DefaultRefreshCLI()
	invalid_exempt_map := map[string]string{"-e": "fake_exemptions_option"}
	invalid_exempt_return := invalid_exempt_cli.FromInteractiveMap(invalid_exempt_map)
	if invalid_exempt_return == nil {
		t.Fatalf("Error when applying interactive map to RefreshCLI: invalid success for map %v", invalid_exempt_map)
	}

	invalid_option_cli := DefaultRefreshCLI()
	invalid_option_map := map[string]string{"--fake-opt": "fake_val"}
	invalid_option_return := invalid_option_cli.FromInteractiveMap(invalid_option_map)
	if invalid_option_return == nil {
		t.Fatalf("Error when applying interactive map to RefreshCLI: invalid success for map %v", invalid_option_map)
	}
}

// TODO: Add a deluge of addtnl. test cases testing things like different path positions and locations,
// invalid inputs, bunch of other ish
func TestParseArgs(t *testing.T) {
	default_config := DefaultRefreshCLI()
	successful_input := []string{
		"-p", "--no-pull", "--opt1=val1", "-o=val2", "--opt3", "val3", "/home/path", "-o4", "--val4",
	}
	expected_1 := []string{"-p", "--no-pull"}
	expected_2 := map[string]string{
		"--opt1": "val1",
		"-o":     "val2",
		"--opt3": "val3",
		"path":   "/home/path",
		"-o4":    "--val4",
	}
	var expected_3 error = nil
	result_1, result_2, result_3 := ParseArgs(successful_input, &default_config)
	if !reflect.DeepEqual(result_1, expected_1) || !reflect.DeepEqual(result_2, expected_2) || !reflect.DeepEqual(result_3, expected_3) {
		fatal_str := ""
		if !reflect.DeepEqual(result_1, expected_1) {
			fatal_str += fmt.Sprintf("NonInput List differs from expected: R:%v != E:%v\n", result_1, expected_1)
		}
		if !reflect.DeepEqual(result_2, expected_2) {
			fatal_str += fmt.Sprintf("InteractiveMap differs from expected: R:%v != E:%v\n", result_2, expected_2)
		}
		if !reflect.DeepEqual(result_3, expected_3) {
			fatal_str += fmt.Sprintf("Unexpected Error: %v", result_3)
		}
		t.Fatal(fatal_str)
	}

}

// TODO: Replace RefreshCLI struct usage with dummy CLIConfig type to mock other CLIConfig methods
// 2 quick options - convert from method on struct to method taking pointer
// OR - make ToNonInputOptions() a func() field of struct (Interesting technique)
func TestApplyCLIInputs(t *testing.T) {
	// Successful case - without any inputs to configure, should succeed
	empty_noninput_vals := []string{}
	empty_interactive_map := map[string]string{}
	empty_cli := DefaultRefreshCLI()
	default_config := DefaultRefreshCLI()
	empty_apply_error := (&empty_cli).ApplyCLIInputs(empty_noninput_vals, empty_interactive_map)
	if empty_apply_error != nil {
		t.Fatal("Empty inputs should have succeeded on ApplyCLIInputs but test failed")
	} else if empty_cli != default_config {
		t.Fatalf("Result config did not match expected (default) config: R:%v E:%v", empty_cli, default_config)
	}

	// Failing case - Return early from invalid noninput option
	invalid_noninput_vals := []string{"-p", "--fake-noninput-opt"}
	failing_cli := DefaultRefreshCLI()
	invalid_apply_error := (&failing_cli).ApplyCLIInputs(invalid_noninput_vals, empty_interactive_map)
	if invalid_apply_error == nil {
		t.Fatal("Invalid options --fake-noninput-opt not caught, no error raised!")
	}

	// Check that ApplyCLIInputs does mutate properly
	non_default_noninputs := []string{"--propagate"}
	non_default_interactive_map := map[string]string{"--delta": "restore"}
	non_default_cli := DefaultRefreshCLI()
	// Set expected result
	expected_non_default := DefaultRefreshCLI()
	expected_non_default.TrackedFilesAction = "restore"
	expected_non_default.Propagate = true
	non_default_error := (&non_default_cli).ApplyCLIInputs(non_default_noninputs, non_default_interactive_map)
	if non_default_error != nil {
		t.Fatalf("Unexpected error when parsing valid inputs: %v", non_default_error)
	} else if default_config == non_default_cli {
		t.Fatalf("non_default_cli not mutated, retained default values: %v", non_default_cli)
	} else if non_default_cli != expected_non_default {
		t.Fatalf("Result config differs from expected: R:%v E:%v", non_default_cli, expected_non_default)
	}
}

func TestGetCLIArgs(t *testing.T) {
	// Setup os.Args "mocking"
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	mockArgs := []string{"git-refresh.exe", "refresh", "--some", "opt"}
	os.Args = mockArgs

	expected := []string{"refresh", "--some", "opt"}
	result := GetCLIArgs()
	if !slices.Equal(result, expected) {
		t.Fatalf("Expected differs from args result: R:%v E%v", result, expected)
	}
}

func TestGetGitRefreshConfig(t *testing.T) {
	// Setup os.Args "mocking"
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	mockArgs := []string{"fake-path"}
	os.Args = mockArgs

	expected := DefaultRefreshCLI()
	cwd, _ := os.Getwd()
	expected.Path = cwd
	result, get_err := GetGitRefreshConfig()
	if !reflect.DeepEqual(expected, *result) {
		t.Fatalf("Expected differs from args result: R:%v E%v", result, expected)
	} else if get_err != nil {
		t.Fatalf("Unexpected error in default case: %v", get_err)
	}
}
