package main

import (
	"fmt"
	"io"
	"os/exec"
	fp "path/filepath"
	"strings"
	"time"
)

type GitOpsDriver interface {
	RunGit(command []string) (string, error)
}

// Struct to encapsulate and standardize git command execution.
type GitDirDriver struct {
	GitDir string
}

// Given a git command, executes command and returns
// stdout and stderr as a string and any associated error
func (c *GitDirDriver) RunGit(command []string) (string, error) {
	git_command := exec.Command(command[0], command[1:]...)
	git_command.Dir = c.GitDir
	git_output, git_err := git_command.CombinedOutput()
	git_std_oe := string(git_output)
	return git_std_oe, git_err
}

// Get branch name, returns branch name and error
// If error is not nil, string will be branch name
func GetGitBranch(gitter GitOpsDriver) (string, error) {
	branch_cmd_str := []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}
	branch_output, branch_err := gitter.RunGit(branch_cmd_str)
	return branch_output, branch_err
}

// Retrieve list of all git-stored files with an error in case this operation fails
func GetGitTrackedFiles(gitter GitOpsDriver, branch string) ([]string, error) {
	// git_ls := exec.Command("git", "ls-tree", "-r", git_branch, "--name-only")
	ls_cmd_str := []string{"git", "ls-files"}
	raw_out, ls_err := gitter.RunGit(ls_cmd_str)
	if ls_err != nil {
		return nil, ls_err
	}
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
func GitStash(gitter GitOpsDriver) error {
	stash_msg := fmt.Sprintf(
		"git refresh - stashing current edits to git-tracked files - %s",
		time.Now().Format("2006-01-02 15:04:05"),
	)
	fmt.Println(stash_msg)
	stash_cmd_str := []string{"git", "stash", "-m", stash_msg}
	_, stash_err := gitter.RunGit(stash_cmd_str)
	return stash_err
}

func GitRestore(gitter GitOpsDriver) error {
	restore_cmd_str := []string{"git", "restore", "."}
	_, restore_err := gitter.RunGit(restore_cmd_str)
	return restore_err
}

func GitPull(gitter GitOpsDriver) error {
	pull_cmd_str := []string{"git", "pull"}
	_, pull_err := gitter.RunGit(pull_cmd_str)
	if pull_err != nil {
		fmt.Println("Failure: Error when pulling from git")
	}
	return pull_err
}

type GitRefreshDriver struct {
	Config     RefreshCLI
	Git        GitOpsDriver
	Writer     io.Writer
	GitDir     string
	RecycleBin string
}

func CreateGitRefreshDriver(config RefreshCLI, writer io.Writer, recycle string) GitRefreshDriver {
	// NOTE: Validate that path is a real git dir here?
	git_inst := GitDirDriver{config.Path}
	return GitRefreshDriver{
		Config:     config,
		Git:        &git_inst,
		Writer:     writer,
		GitDir:     config.Path,
		RecycleBin: recycle,
	}
}

func (d *GitRefreshDriver) PullAction() error {
	if d.Config.Pull {
		return GitPull(d.Git)
	}
	return nil
}

func (d *GitRefreshDriver) DeltaAction() error {
	switch d.Config.TrackedFilesAction {
	case "stash":
		return GitStash(d.Git)
	case "restore":
		return GitRestore(d.Git)
	case "null":
		return nil
	}
	return nil
}

// By default - need to implement safe mode execution
// - anything stored in git gets stashed
// - Anything else immediately into recycling bin (what about collisions if the same project has the same directory name but just exists is nested down 2 different directory paths?)
// - git pull on current branch
func GitRefreshSingleRepo(git_refresh GitRefreshDriver) error {
	git_dir := git_refresh.GitDir
	recycle_bin := git_refresh.RecycleBin

	////// Retrieve git metadata
	git_branch, branch_err := GetGitBranch(git_refresh.Git)
	if branch_err != nil {
		return branch_err
	}
	// TODO: Add method to git_refresh to include verbosity in the operation
	fmt.Fprintln(git_refresh.Writer, "Refresh operating on ", git_branch, " on path ", git_dir, "...")

	// Operate on git-tracked files with a delta (non-op, restore, or stash)
	delta_err := git_refresh.DeltaAction()
	if delta_err != nil {
		fmt.Fprintln(
			git_refresh.Writer,
			"Error during git delta action: ",
			git_refresh.Config.TrackedFilesAction,
		)
		return delta_err
	}

	fmt.Fprintln(git_refresh.Writer, "Finding git-tracked files...")
	git_list, ls_err := GetGitTrackedFiles(git_refresh.Git, git_branch)
	if ls_err != nil {
		return ls_err
	}

	fmt.Println("Print git list:")
	for _, git_file := range git_list {
		fmt.Println(git_file)
	}
	//	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))

	dir_contents, walk_err := GetAllDirContents(git_dir)
	if walk_err != nil {
		fmt.Fprintln(git_refresh.Writer, "Error when generating list of directory contents.")
		return walk_err
	}

	exemption_path := fp.Join(git_dir, ".gitrefresh")
	fmt.Fprintln(git_refresh.Writer, "Generating git refresh exemption list...")
	exempt_files, exempt_dirs, exempt_err := GetRefreshExemptions(exemption_path)
	if exempt_err != nil {
		return exempt_err
	}
	// fmt.Println("Print exempt files")
	// for _, name := range exempt_files {
	// 	fmt.Println(name)
	// }
	// fmt.Println("Print exempt dirs")
	// for _, name := range exempt_dirs {
	// 	fmt.Println(name)
	// }

	// TODO: Remove
	fmt.Println("Calculating deletion list")
	delete_list, get_deletes_err := GetDeletionList(dir_contents, git_list, exempt_files, exempt_dirs)
	if get_deletes_err != nil {
		fmt.Fprintln(git_refresh.Writer, "Error when determining file deletion list.")
		return get_deletes_err
	}

	//	delete_list_check, _ := SaferGetDeletionList(dir_contents, git_list, exempt_files, exempt_dirs)
	//	fmt.Println("Correctness check: ", slices.Equal(delete_list, delete_list_check))
	for _, file_name := range delete_list {
		fmt.Println(file_name)
	}

	fmt.Fprintln(git_refresh.Writer, "Moving files to recycling bin...")
	delete_err := RecycleFiles(delete_list, git_dir, recycle_bin)
	if delete_err != nil {
		return delete_err
	}

	pull_err := git_refresh.PullAction()
	if pull_err != nil {
		fmt.Fprintln(git_refresh.Writer, "Error during git pull action.")
		return pull_err
	}

	fmt.Fprintln(git_refresh.Writer, "Operation complete, deleted files moved to ", recycle_bin)
	return nil
}
