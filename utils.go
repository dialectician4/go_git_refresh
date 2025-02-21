package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

type CLIConfig interface {
	ToNonInputOptions() []string
	//ToInteractiveOptions() []string
	FromNonInputList(flags []string) error
	FromInteractiveMap(flag_map map[string]string) error
}

// TrackedFilesAction should be in ["stash", "restore", "null"] (default as stash)
// Pull, Propagate should be T/F (defualtsa as T and F, respectively)
// ExemptionSrc should be ["(git)refresh", "(git)ignore"] (default as refresh)
// Path should be interpretable as . or path (default as .)

type RefreshCLI struct {
	Path               string
	TrackedFilesAction string
	Propagate          bool
	ExemptionsSrc      string
	Pull               bool
}

func DefaultRefreshCLI() RefreshCLI {
	return RefreshCLI{Path: ".", TrackedFilesAction: "null", Propagate: false, ExemptionsSrc: "refresh", Pull: false}
}

func (r *RefreshCLI) ToNonInputOptions() []string {
	output := []string{"--propagate", "-p", "--pull", "--no-pull", "-gp", "-ngp"}
	return output
}

func (r *RefreshCLI) FromNonInputList(flags []string) error {
	for _, flag := range flags {
		switch flag {
		case "--propagate", "-p":
			r.Propagate = true
		case "--pull", "-gp":
			r.Pull = true
		case "--no-pull", "-ngp":
			r.Pull = false
		default:
			return errors.New(fmt.Sprintln("Flag not recognized: ", flag))
		}
	}
	return nil
}

func (r *RefreshCLI) FromInteractiveMap(flag_map map[string]string) error {
	deltaParse := func(s string) (string, error) {
		switch s {
		case "stash", "s":
			return "stash", nil
		case "restore", "r":
			return "restore", nil
		case "null", "n":
			return "null", nil
		default:
			return "", errors.New(fmt.Sprintln("Unrecognized value for delta option: ", s))
		}
	}

	exemptsParse := func(s string) (string, error) {
		switch s {
		case "git-refresh", "r":
			return "refresh", nil
		case "git-ignore", "i":
			return "ignore", nil
		default:
			return "", errors.New(fmt.Sprintln("Unrecognized value for exemptions option: ", s))
		}
	}
	for opt, opt_val := range flag_map {
		switch opt {
		case "--delta", "-d":
			delta_val, delta_err := deltaParse(opt_val)
			if delta_err != nil {
				return delta_err
			}
			r.TrackedFilesAction = delta_val
		case "--exemptions", "-e":
			exempt_val, exempt_err := exemptsParse(opt_val)
			if exempt_err != nil {
				return exempt_err
			}
			r.ExemptionsSrc = exempt_val
		default:
			return errors.New(fmt.Sprintln("Flag not recognized: ", opt))
		}
	}
	return nil
}

func CLIParse(args []string, config *CLIConfig) error {
	noninput_flags := (*config).ToNonInputOptions()
	noninput_vals := []string{}
	//interactive_flags := (*config).ToInteractiveOptions()
	interactive_map := map[string]string{"path": ""}
	remainders := []string{}
	// Take flags noot taking inputs and extract
	for _, arg := range args {
		if slices.Contains(noninput_flags, arg) {
			noninput_vals = append(noninput_vals, arg)
		} else {
			remainders = append(remainders, arg)
		}
	}
	// Split any options on = which were concatenated with their value
	split_remainders := []string{}
	for _, arg := range remainders {
		if strings.Contains(arg, "=") {
			pairs := strings.Split(arg, "=")
			split_remainders = append(split_remainders, pairs[0], pairs[1])
		} else {
			split_remainders = append(split_remainders, arg)
		}
	}
	// Now apply unified sliding window method for reading input pairs
	used_options := []string{}
	for i := 0; i <= len(split_remainders)-1; i++ {
		if strings.HasPrefix(split_remainders[i], "-") &&
			!slices.Contains(used_options, split_remainders[i+1]) &&
			!slices.Contains(used_options, split_remainders[i]) {
			interactive_map[split_remainders[i]] = split_remainders[i+1]
			used_options = append(used_options, split_remainders[i], split_remainders[i+1])
		}
	}

	// In this case, exactly 1 should be unused (the path value)
	if len(split_remainders) == len(used_options)+1 {
		for _, val := range split_remainders {
			if !slices.Contains(used_options, val) {
				interactive_map["path"] = val
				break
			}
		}
		return errors.New(fmt.Sprintf("Issue finding unambigious path term in subset %v of cli args %v\n", split_remainders, args))
	} else if (len(split_remainders) > len(used_options)+1) || (len(split_remainders) < len(used_options)) {
		return errors.New(fmt.Sprintf("Issue finding unambigious path term in subset %v of cli args %v\n", split_remainders, args))
	}

	parse_noninputs_err := (*config).FromNonInputList(noninput_vals)
	if parse_noninputs_err != nil {
		return parse_noninputs_err
	}
	parse_interactive_err := (*config).FromInteractiveMap(interactive_map)
	return parse_interactive_err
}

// TrackedFilesAction should be in ["stash", "restore", "null"] (default as stash)
// Pull, Propagate should be T/F (defualtsa as T and F, respectively)
// ExemptionSrc should be ["(git)refresh", "(git)ignore"] (default as refresh)
// Path should be interpretable as . or path (default as .)

func GetCLIArgs() []string {
	args := os.Args[1:]
	return args
}

//func StructureCLIArgs(cli_inputs []string) (map[string]string, error) {
//	cli_inputs_original := make([]string, len(cli_inputs))
//	copy(cli_inputs_original, cli_inputs)
//	options_map := map[string]string{"path": ""}
//	option_set := [2]string{"", ""} // (option_specifier, option_value) tuple
//
//	for i := 0; i <= len(cli_inputs_original) - 1; i++ {
//		if strings.Contains(cli_inputs[i], "-") && strings.Contains(cli_inputs[i+1], "-") {
//			options_map[cli_inputs[i]] == ""
//		} else if  strings.Contains(cli_inputs[i], "-") && !strings.Contains(cli_inputs[i+1], "-") {
//			optionsoptions_map[clicli_inputs[i]] == cli_cli_inputs[i+1]
//		}
//	}
//
//	for len(cli_inputs) > 0 {
//		front := cli_inputs[0]
//		cli_inputs = cli_inputs[1:]
//		// Args should either be opt specifiers (with "-") or opt values (without "-")
//		// Each time encountering a "-", load opt specifier into option_set. For opt value,load into option_set 2nd elt.
//		// If encountering opt_val without an opt_specifier already set, set as the value of path
//		if strings.Contains(front, "-") && (len(option_set[0]) == 0) {
//			option_set[0] = front
//		} else if !strings.Contains(front, "-") && (len(option_set[0]) > 0) {
//			option_set[1] = front
//		} else if !strings.Contains(front, "-") && (len(option_set[0]) == 0) && (len(options_map["path"]) == 0) {
//			options_map["path"] = front
//		} else {
//			return errors.New(
//				"Error: parsing of " +
//					strings.Join(cli_inputs_original, " ") +
//					" failed")
//		}
//		if (len(option_set[0]) > 0) && (len(option_set[1]) > 0) {
//			options_map[option_set[0]] = option_set[1]
//			option_set = [2]string{"", ""}
//		} else if (len(option_set[0]) > 0) && (len(option_set[1]) == 0) && strings.Contains {
//	}
//
//	return options_map, nil
//
//}
