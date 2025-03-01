package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"

	//"os/exec"
	fp "path/filepath"
)

// NOTE: IDEA: Command to easily retrieve a file in the recycle bin
//func use_utils() {
//	GetRefreshArgs()
//}

func main() {
	// config, config_error :=GetGitRefreshConfig()
	// if config_error != nil {
	// 	// TODO: Log error
	// 	os.Exit(1)
	// }
	GitRefreshMainProcedure()
}

// By default - need to implement safe mode execution
// - anything stored in git gets stashed
// - Anything else immediately into recycling bin (what about collisions if the same project has the same directory name but just exists is nested down 2 different directory paths?)
// - git pull on current branch
func GitRefreshMainProcedure() {
	////// Setup git refresh recycling bin if not already setup
	home, home_err := os.UserHomeDir()
	git_dir, cwd_err := os.Getwd()
	CheckExit(home_err)
	CheckExit(cwd_err)
	// Create recycling directory if it does not exist
	recycle_dir := fp.Join(home, ".git_refresh_rcycl")
	recycle_err := recycleSetup(recycle_dir)
	CheckExit(recycle_err)

	////// Retrieve git metadata
	git_branch, branch_err := GetGitBranch(git_dir)
	CheckExit(branch_err)

	fmt.Println(git_branch, " on path ", git_dir)

	fmt.Println("Fetching git files")
	git_list, ls_err := GetGitTrackedFiles(git_dir, git_branch)
	CheckExit(ls_err)

	fmt.Println("Print git list:")
	for _, git_file := range git_list {
		fmt.Println(git_file)
	}
	//	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))

	// Stash git tracked files to operate only on non-git files
	//stash_err := GitStash(git_dir)
	//CheckExit(stash_err)

	dir_contents, walk_err := GetAllDirContents(git_dir)
	CheckExit(walk_err)

	exemption_path := fp.Join(git_dir, ".gitrefresh")
	exempt_files, exempt_dirs, exempt_err := GetRefreshExemptions(exemption_path)
	CheckExit(exempt_err)
	fmt.Println("Print exempt files")
	for _, name := range exempt_files {
		fmt.Println(name)
	}
	fmt.Println("Print exempt dirs")
	for _, name := range exempt_dirs {
		fmt.Println(name)
	}

	fmt.Println("Calculating deletion list")
	delete_list, get_deletes_err := GetDeletionList(dir_contents, git_list, exempt_files, exempt_dirs)
	CheckExit(get_deletes_err)

	//	delete_list_check, _ := SaferGetDeletionList(dir_contents, git_list, exempt_files, exempt_dirs)
	//	fmt.Println("Correctness check: ", slices.Equal(delete_list, delete_list_check))
	for _, file_name := range delete_list {
		fmt.Println(file_name)
	}

	delete_err := DeleteFiles(delete_list, git_dir, recycle_dir)
	CheckExit(delete_err)

	recycle_bin := fp.Join(recycle_dir, path.Base(git_dir))

	pull_err := GitPull(git_dir)
	CheckExit(pull_err)

	fmt.Println("Operation complete, deleted files moved to ", recycle_bin)
	os.Exit(0)

}

// Idempotent way of setting up the recycling directory if it does not exist.
// Returns non-nil error if directory if directory check fails or if mkdir fails
func recycleSetup(recycle_dir string) error {
	// Create directory if directory does not exist
	_, recycle_dir_err := os.Stat(recycle_dir)
	if errors.Is(recycle_dir_err, os.ErrNotExist) {
		mkdir_err := os.MkdirAll(recycle_dir, 0755)
		return mkdir_err
		// Error would propagate properly anyways
		//		if mkdir_err != nil {
		//			// Find a way to compose errors
		//			log.Println("Error while setting up the git_refresh recycle bin: ", mkdir_err)
		//			// return mkdir_err
		//		}
	}
	return recycle_dir_err
}

// Get branch name, returns branch name and error
// If error is not nil, string will be branch name
func GetGitBranch(git_dir string) (string, error) {
	git_branch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	git_branch.Dir = git_dir
	branch_name := ""
	branch_output, branch_err := git_branch.Output()
	branch_name = string(branch_output)
	return branch_name, branch_err
}

// Retrieve list of all git-stored files with an error in case this operation fails
func GetGitTrackedFiles(git_dir, git_branch string) ([]string, error) {
	//git_ls := exec.Command("git", "ls-tree", "-r", git_branch, "--name-only")
	git_ls := exec.Command("git", "ls-files")
	git_ls.Dir = git_dir
	raw_out, ls_err := git_ls.Output()
	git_tracked_files := strings.Split(string(raw_out), "\n")
	var git_list []string
	for _, file_name := range git_tracked_files {
		abs_path, abs_err := fp.Abs(file_name)
		if abs_err != nil {
			return git_list, abs_err
		}
		git_list = append(git_list, abs_path)
	}
	return git_list, ls_err
}

// TODO: Convert fmt prints to logs

// Stash changes to git-tracked files
func GitStash(git_dir string) error {
	stash_msg := fmt.Sprintf(
		"git refresh - stashing current edits to git-tracked files - %s",
		time.Now().Format("2006-01-02 15:04:05"),
	)
	fmt.Println(stash_msg)
	git_stash := exec.Command("git", "stash", "-m", stash_msg)
	git_stash.Dir = git_dir
	stash_err := git_stash.Run()
	return stash_err
}

func GitPull(git_dir string) error {
	git_pull := exec.Command("git", "pull")
	git_pull.Dir = git_dir
	pull_err := git_pull.Run()
	if pull_err != nil {
		fmt.Println("Failure: Error when pulling from git")
	}
	return pull_err
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

// NOTE: At some point should prolly include a check that the directory is git-managed, or just wait for it to be caught in one of the errors?

func DeleteFiles(delete_list []string, cwd, recycle_dir string) error {
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
