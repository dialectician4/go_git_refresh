package main

import (
	"errors"
	"fmt"
	// "io"
	"io"
	"io/fs"
	"log"
	"os"
	fp "path/filepath"
	"slices"
	"strings"
	"sync"
)

// NOTE: IDEA: Command to easily retrieve a file in the recycle bin
func main() {
	logc := make(chan string, 10)
	write_chan := CreateChannelWriter(logc)
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()
		LogRoutine(os.Stdout, logc)
	}()

	exit_code := GitRefreshMain(write_chan)
	close(logc)
	wg.Wait()
	os.Exit(exit_code)
}

func GitRefreshMain(logger io.Writer) int {
	////// Get stdout as io writer
	git_refresh_writer := logger
	cli_args := GetCLIArgs()
	fmt.Println(cli_args)
	////// Get configurations for git refresh
	config, config_err := GetGitRefreshConfig()
	if config_err != nil {
		fmt.Fprintln(git_refresh_writer, "Error when parsing input to git refresh:\n", config_err)
		return 1
	}
	////// Setup git refresh recycling bin if not already setup
	home, home_err := os.UserHomeDir()
	if home_err != nil {
		fmt.Fprintln(git_refresh_writer, "Error when retrieving home directory:\n", home_err)
		return 1
	}
	// Create recycling directory if it does not exist
	fmt.Println("Path: ", config.Path, ", Base of Path: ", fp.Base(config.Path))
	fmt.Println("test base: ", fp.Base(`\projects\go\go_git_refresh`))
	recycle_bin := fp.Join(home, ".git_refresh_rcycl", fp.Base(config.Path))
	// recycle_err := recycleSetup(recycle_bin)
	// if recycle_err != nil {
	// 	fmt.Fprintln(
	// 		git_refresh_writer,
	// 		"Error when setting up recycle bin at ",
	// 		recycle_bin,
	// 		":\n",
	// 		recycle_err,
	// 	)
	// 	return 1
	// }
	////// Generate driver for GitRefresh procedure
	git_refresh_inst := CreateGitRefreshDriver(*config, git_refresh_writer, recycle_bin)
	refresh_err := GitRefreshSingleRepo(git_refresh_inst)
	if refresh_err != nil {
		fmt.Fprintln(git_refresh_writer, "git refresh early termination due to the following error:", refresh_err)
		return 1
	}

	return 0
}

// Idempotent way of setting up the recycling directory if it does not exist.
// Returns non-nil error if directory if directory check fails or if mkdir fails
func recycleSetup(recycle_dir string) error {
	// Create directory if directory does not exist
	_, recycle_dir_err := os.Stat(recycle_dir)
	if errors.Is(recycle_dir_err, os.ErrNotExist) {
		// NOTE: Is this no-opp return nil if dir already exists?
		mkdir_err := os.MkdirAll(recycle_dir, 0755)
		return mkdir_err
	}
	return recycle_dir_err
}

// Simple command to Exit program if error is non-nil and print first
func CheckExit(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Returns absolute paths to all files in directory
func GetAllDirContents(dir string) ([]string, error) {
	var out_list []string
	var walk_closure fs.WalkDirFunc
	walk_closure = func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Failure accessing path %q: %v\n", path, err)
			return err
		}
		if !info.IsDir() {
			abs_path, abs_err := fp.Abs(path)
			if abs_err != nil {
				return abs_err
			}
			out_list = append(out_list, abs_path)
		}
		return nil
	}
	walk_err := fp.WalkDir(
		dir,
		walk_closure,
	)
	return out_list, walk_err

}

func GetDeletionList(all_files, git_files, exempt_files, exempt_dirs []string) ([]string, error) {
	// Setup filtering set
	var delete_files []string
	keep_files := make(map[string]bool)

	for _, i := range git_files {
		keep_files[i] = true
	}
	for _, j := range exempt_files {
		keep_files[j] = true
	}

	// TODO: Check that exempt_dirs are all absolute
	// TODO: Check all_files is all absolute

	for _, file_name := range all_files {
		_, keep := keep_files[file_name]
		if keep {
			continue
		}

		in_exempt_dir := false
		for _, dir := range exempt_dirs {
			rel_path, rel_err := fp.Rel(dir, file_name)
			if rel_err != nil {
				return delete_files, rel_err
			}
			in_exempt_dir = in_exempt_dir || !strings.HasPrefix(rel_path, ".."+string(fp.Separator))
		}
		if in_exempt_dir {
			continue
		}
		delete_files = append(delete_files, file_name)
	}
	return delete_files, nil

}

func SaferGetDeletionList(all_files, git_files, exempt_files, exempt_dirs []string) ([]string, error) {
	var delete_files []string

	for _, file := range all_files {
		if slices.Contains(git_files, file) {
			continue
		}
		if slices.Contains(exempt_files, file) {
			continue
		}
		in_exempt_dir := false
		for _, dir := range exempt_dirs {
			rel_path, rel_err := fp.Rel(dir, file)
			if rel_err != nil {
				return delete_files, rel_err
			}
			in_exempt_dir = in_exempt_dir || !strings.HasPrefix(rel_path, ".."+string(fp.Separator))
		}
		if in_exempt_dir {
			continue
		}
		delete_files = append(delete_files, file)
	}

	return delete_files, nil
}

// Given .gitrefresh path, returns
// exempt files list, exempt directories list, and a (nil) error
// All file paths in both lists are absolute
func GetRefreshExemptions(exempts_file string) ([]string, []string, error) {
	// TODO: Include separate logic for exemptions in .gitignore
	// Currently only implement for separate .git_refresh file
	var exempt_files []string
	var exempt_dirs []string

	data, read_err := os.ReadFile(exempts_file)
	if read_err != nil {
		log.Println("Error reading exemption file ", exempts_file)
		return exempt_files, exempt_dirs, read_err
	}
	lines := strings.Split(string(data), "\n")
	lines = append(lines, ".git")
	var raw_path string
	for _, line := range lines {
		// Read Path on each line, clean and remove comments
		raw_path = ""
		comment_start := strings.Index(line, "#")
		if comment_start < 0 {
			raw_path = line
		} else {
			raw_path = line[:comment_start]
		}
		trim_path := strings.TrimSpace(raw_path)
		// Skip all-comment lines or empty lines
		if len(trim_path) == 0 {
			continue
		}

		abs_path, abs_err := fp.Abs(trim_path)
		if abs_err != nil {
			log.Println("Error resolving absolute path for ", trim_path)
			return exempt_files, exempt_dirs, abs_err
		}
		pathInfo, path_err := os.Stat(abs_path)
		if errors.Is(path_err, os.ErrNotExist) {
			continue
		} else if path_err != nil {
			log.Println("Error stating ")
			log.Println(abs_path)
			log.Println(" from trimmed ")
			log.Println("filename:", trim_path, ":endfile:ln:", len(trim_path))
			return exempt_files, exempt_dirs, path_err
		}
		// Add path to dir list if is dir, to file list if is file
		if pathInfo.IsDir() {
			exempt_dirs = append(exempt_dirs, abs_path)
		} else {
			exempt_files = append(exempt_files, abs_path)
		}
	}

	return exempt_files, exempt_dirs, nil

}

// NOTE: At some point should include a check that the directory is git-managed, or just wait for it to be caught in one of the errors?

func RecycleFiles(delete_list []string, cwd, recycle_dir string, skip_recycle bool) error {
	if skip_recycle {
		fmt.Println("Recycling/file deletion skipped.")
		return nil
	}
	recycle_dir = fp.Dir(recycle_dir)
	stemming_path := fp.Dir(cwd)
	for _, src_path := range delete_list {
		remainder, stem_err := fp.Rel(stemming_path, src_path)
		if stem_err != nil {
			return stem_err
		}
		dst_path := fp.Join(recycle_dir, remainder)
		fmt.Println("Target dir: ", fp.Dir(dst_path), " from path ", dst_path)
		mkdir_err := os.MkdirAll(fp.Dir(dst_path), 0755)
		if mkdir_err != nil {
			return mkdir_err
		}
		delete_err := os.Rename(src_path, dst_path)
		if delete_err != nil {
			return delete_err
		}
	}

	return nil
}
