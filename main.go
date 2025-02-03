package main

import (
	"errors"
	"fmt"
	"io/fs"
	"time"
	//	"log"
	"os"
	"os/exec"
	"strings"

	//"os/exec"
	fp "path/filepath"
)

// NOTE: IDEA: Command to easily retrieve a file in the recycle bin

// By default - need to implement safe mode execution
// - anything stored in git gets stashed
// - Anything else immediately into recycling bin (what about collisions if the same project has the same directory name but just exists is nested down 2 different directory paths?)
// - git pull on current branch
func main() {
	////// Setup git refresh recycling bin if not already setup
	home, home_err := os.UserHomeDir()
	git_dir, cwd_err := os.Getwd()
	CheckExit(home_err)
	CheckExit(cwd_err)
	// Create recycling directory if it does not exist
	recycle_dir := fp.Join(home, "git_refresh_rcycl")
	recycle_err := recycleSetup(recycle_dir)
	CheckExit(recycle_err)

	////// Retrieve git metadata
	git_branch, branch_err := GetGitBranch(git_dir)
	CheckExit(branch_err)

	fmt.Println(git_branch, " on path ", git_dir)

	git_list, ls_err := GetGitTrackedFiles(git_dir, git_branch)
	CheckExit(ls_err)

	fmt.Println(git_list)
	//	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))

	// Stash git tracked files to operate only on non-git files
	//stash_err := Stash(git_dir)
	//CheckExit(stash_err)

	dir_contents, walk_err := GetAllDirContents(git_dir)
	CheckExit(walk_err)
	for _, file_name := range dir_contents {
		fmt.Println(file_name)
	}
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
	return git_tracked_files, ls_err
}

// Stash changes to git-tracked files
func Stash(git_dir string) error {
	stash_msg := fmt.Sprintf(
		"git refresh - stashing current edits to git-tracked files - %s",
		time.Now().Format("2006-01-02 15:04:05"),
	)
	fmt.Println(stash_msg)
	git_stash := exec.Command("git", "stash", "-m", stash_msg)
	stash_err := git_stash.Run()
	return stash_err
}

// Simple command to Exit program if error is non-nil and print first
func CheckExit(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func GetAllDirContents(dir string) ([]string, error) {
	var out_list []string
	var walk_closure fs.WalkDirFunc
	walk_closure = func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Failure accessing path %q: %v\n", path, err)
			return err
		}
		if !info.IsDir() {
			out_list = append(out_list, path)
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
	//           A          B          C
	// mA = min(A) = A-B-C where mA is a non-negative integer
	// time cost to check if deletion files in either git files or exempt files list (which we know they are not):
	// mA*(B + C) = O(mA*B) + O(mA*C)
	// time cost to check if first we hash the git files and exempt files list
	// c1*(B + C) [time-cost to create a hash map from both lists combined] + mA*c2 (mA many constant-time checks if element is in hash map) = O(B) + O(C) + O(mA)

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
		exempt := keep
		for _, dir := range exempt_dirs {
			rel_path, rel_err := fp.Rel(dir, file_name)
			if rel_err != nil {
				return delete_files, rel_err
			}
			exempt = exempt || !strings.HasPrefix(rel_path, ".."+string(fp.Separator))
		}
		if exempt {
			delete_files = append(delete_files, file_name)
			continue
		}

	}
	return delete_files, nil

}

// NOTE: At some point should prolly include a check that the directory is git-managed, or just wait for it to be caught in one of the errors?
