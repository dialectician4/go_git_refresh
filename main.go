package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		LogRoutine(os.Stdout, logc)
	}()

	exit_code := GitRefreshMain(write_chan)
	close(logc)
	wg.Wait()
	os.Exit(exit_code)
}

func GitRefreshMain(logger io.Writer) int {
	////// Get configurations for git refresh
	config, config_err := GetGitRefreshConfig()
	fmt.Fprintln(logger, "git refresh repos...")
	if config_err != nil {
		fmt.Fprintln(logger, "Error when parsing input to git refresh:\n", config_err)
		return 1
	}
	git_refresh_targets := config.FindRefreshTargets()
	if git_refresh_targets.IsErr() {
		fmt.Fprintln(logger, git_refresh_targets.UnwrapErr())
		return 1
	}
	refresh_list := git_refresh_targets.Unwrap()
	////// Setup git refresh recycling bin if not already setup
	home, home_err := os.UserHomeDir()
	if home_err != nil {
		fmt.Fprintln(logger, "Error when retrieving home directory:\n", home_err)
		return 1
	}

	result_chan := make(chan RefreshResult, len(refresh_list))
	var wg sync.WaitGroup
	for i, repo := range refresh_list {
		wg.Add(1)
		go func() {
			defer wg.Done()
			single_config := config.CloneWNewPath(repo)
			recycle_bin := fp.Join(home, ".git_refresh_rcycl", fp.Base(single_config.Path))
			git_refresh_inst := CreateGitRefreshDriver(single_config, logger, recycle_bin, i)
			refresh_err := GitRefreshSingleRepo(git_refresh_inst)
			refresh_result := AsRefreshResult(repo, refresh_err)
			result_chan <- refresh_result
		}()
	}

	result_list := make([]RefreshResult, len(refresh_list))

	wg.Wait()
	errors := 0
	for range len(refresh_list) {
		result := <-result_chan
		result_name := result.Repo
		success := !result.Res.IsErr()
		result_list = append(result_list, result)
		if success {
			fmt.Fprintln(logger, "Repo ", result_name, " refreshed successfully")
		} else {
			errors += 1
			fmt.Fprintln(
				logger, "Repo ", result_name,
				" failed to refresh: error ", result.Res.UnwrapErr())
		}
	}

	if errors == 0 {
		fmt.Fprintln(logger, "All repos refreshed successfully!")
		return 0
	} else {
		fmt.Fprintln(logger, "Failed to refresh", errors, "repos.")
		return 1
	}
}

type RefreshResult struct {
	Repo string
	Res  Result[Empty]
}

func AsRefreshResult(name string, result error) RefreshResult {
	return RefreshResult{Res: Result[Empty]{err: result}, Repo: name}
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

func GetNestedDirs(dir string) ([]string, error) {
	var out_list []string
	var walk_closure fs.WalkDirFunc
	walk_closure = func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Failure accessing path %q: %v\n", path, err)
			return err
		}
		if info.IsDir() {
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

// Filters slice of file paths and leaves only top-level git repo directories
func CheckGitDirs(contents []string) Result[[]string] {
	slices.SortFunc(
		contents,
		func(a, b string) int {
			return cmp.Compare(len(a), len(b))
		},
	)

	// TODO: Change this to Result[bool]?
	git_repo_check := func(dir string) bool {
		git_status := exec.Command("git", "status")
		git_status.Dir = dir
		git_output, _ := git_status.CombinedOutput()
		output := string(git_output)
		return strings.Contains(output, "On branch ")
	}
	repo_list := []string{}
	path_ct := len(contents)
	for range path_ct {
		if len(contents) == 0 {
			return Ok(repo_list)
		}
		path := contents[0]
		status := git_repo_check(path)
		if status {
			repo_list = append(repo_list, path)
			cleaned_subset := RemoveSubpaths(path, contents)
			if cleaned_subset.IsErr() {
				return *cleaned_subset.Context("Could not correctly purge subpaths under git repo")
			}
			contents = cleaned_subset.Unwrap()
		} else {
			contents = contents[1:]
		}

	}
	return Ok(repo_list)

}

func RemoveSubpaths(parent string, path_list []string) Result[[]string] {
	result := []string{}
	for _, path := range path_list {
		issubpath := IsSubPath(parent, path)
		if issubpath.IsErr() {
			return Transmute[bool, []string](issubpath)
		}
		if !issubpath.Unwrap() {
			result = append(result, path)
		}
	}
	return Ok(result)
}

func IsSubPath(parent, sub string) Result[bool] {
	up := ".." + string(os.PathSeparator)
	rel, err := fp.Rel(parent, sub)
	if err != nil {
		rerr := Err[bool](err)
		return *rerr.Context(
			"Error resolving file paths between " + parent + " and " + sub,
		)
	}
	issubpath := !strings.HasPrefix(rel, up) && rel != ".."
	return Ok(issubpath)
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
func GetRefreshExemptions(repo string, exempts_file string) ([]string, []string, error) {
	// TODO: Include separate logic for exemptions in .gitignore
	// Currently only implement for separate .git_refresh file
	var exempt_files []string
	var exempt_dirs []string

	data, read_err := os.ReadFile(exempts_file)
	if os.IsNotExist(read_err) {
		return []string{}, []string{}, nil
	} else if read_err != nil {
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

		abs_path := fp.Join(repo, trim_path)
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

func RecycleFiles(logger func(args ...any), delete_list []string, cwd, recycle_dir string) error {
	recycle_dir = fp.Dir(recycle_dir)
	stemming_path := fp.Dir(cwd)
	for _, src_path := range delete_list {
		remainder, stem_err := fp.Rel(stemming_path, src_path)
		if stem_err != nil {
			return stem_err
		}
		dst_path := fp.Join(recycle_dir, remainder)
		logger("Target dir: ", fp.Dir(dst_path), " from path ", dst_path)
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
