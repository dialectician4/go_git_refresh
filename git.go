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
	RepoNumber int
}

func CreateGitRefreshDriver(config RefreshCLI, writer io.Writer, recycle string, index int) GitRefreshDriver {
	git_inst := GitDirDriver{config.Path}
	return GitRefreshDriver{
		Config:     config,
		Git:        &git_inst,
		Writer:     writer,
		GitDir:     config.Path,
		RecycleBin: recycle,
		RepoNumber: index,
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

func (d *GitRefreshDriver) GetLogger() func(args ...any) {
	repo_name := fp.Base(d.Config.Path)
	color, stop := TerminalColors(d.RepoNumber)
	logger := func(args ...any) {
		prints := []any{fmt.Sprintf("%s%s:", color, repo_name)}
		prints = append(prints, args...)
		prints = append(prints, stop)
		fmt.Fprintln(d.Writer, prints...)
	}
	return logger

}

// By default - need to implement safe mode execution
// - anything stored in git gets stashed
// - Anything else immediately into recycling bin (what about collisions if the same project has the same directory name but just exists is nested down 2 different directory paths?)
// - git pull on current branch
func GitRefreshSingleRepo(git_refresh GitRefreshDriver) error {
	git_dir := git_refresh.GitDir
	recycle_bin := git_refresh.RecycleBin
	logger := git_refresh.GetLogger()
	recycle_err := recycleSetup(recycle_bin)
	if recycle_err != nil {
		logger(
			"Error when setting up recycle bin at ",
			recycle_bin,
			":\n",
			recycle_err,
		)
		return recycle_err
	}

	////// Retrieve git metadata
	git_branch, branch_err := GetGitBranch(git_refresh.Git)
	if branch_err != nil {
		return branch_err
	}
	// TODO: Add method to git_refresh to include verbosity in the operation
	logger("Refresh operating on ", git_branch, " on path ", git_dir, "...")

	// Operate on git-tracked files with a delta (non-op, restore, or stash)
	logger("Git action on delta files: ", git_refresh.Config.TrackedFilesAction)
	delta_err := git_refresh.DeltaAction()
	if delta_err != nil {
		logger(
			"Error during git delta action: ",
			git_refresh.Config.TrackedFilesAction,
		)
		return delta_err
	}

	logger("Finding git-tracked files...")
	git_list, ls_err := GetGitTrackedFiles(git_refresh.Git, git_branch)
	if ls_err != nil {
		return ls_err
	}

	dir_contents, walk_err := GetAllDirContents(git_dir)
	if walk_err != nil {
		logger("Error when generating list of directory contents.")
		return walk_err
	}

	exemption_path := fp.Join(git_dir, ".gitrefresh")
	logger("Generating git refresh exemption list...")
	exempt_files, exempt_dirs, exempt_err := GetRefreshExemptions(exemption_path)
	if exempt_err != nil {
		return exempt_err
	}

	delete_list, get_deletes_err := GetDeletionList(dir_contents, git_list, exempt_files, exempt_dirs)
	if get_deletes_err != nil {
		logger("Error when determining file deletion list.")
		return get_deletes_err
	}
	logger("Found", len(delete_list), "files to delete")

	logger("Moving files to recycling bin...")
	delete_err := RecycleFiles(logger, delete_list, git_dir, recycle_bin, git_refresh.Config.SkipRecycle)
	if delete_err != nil {
		return delete_err
	}

	is_pulling := Ternary(git_refresh.Config.Pull, "P", "Not p")
	logger(is_pulling + "ulling updates from repo.")
	pull_err := git_refresh.PullAction()
	if pull_err != nil {
		logger("Error during git pull action.")
		return pull_err
	}

	logger("Operation complete, deleted files moved to ", recycle_bin)
	return nil
}
