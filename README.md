git refresh
------------
git refresh is a git plugin which allows you to simultaneously remove
untracked files from your repository and pull from git to help
synchronize the state of your repo with remote.

git refresh allows you to configure how exactly to execute the refresh:
whether to pull latest updates or not, whether to stash, restore, or
leave deltas to git-tracked files untouched, whether to not delete files
at all, or even whether to apply git refresh on all nested sub-directories!
Uses goroutines to parallelize execution on subdirectories.

### Why git refresh?
Have you ever worked on a project where one process or another generates
a variety of cache files or otherwise changes your state? And you leave
these files be until 2 people start getting different results on some
test and you hear "It works on my machine?" I've seen this happen and the
issue persists until someone deletes their repo altogether and reclones
and the issue is revealed to lie in caches or other non-git
tracked files.

git refresh aims to be a user-friendly tool to get your repo synchronized
with git to help aid in situations like the above or simply to keep your
local clean and up to date. git refresh clears out your untracked files
and executes a pull to avoid situations such as having to re-clone
outright.

For maximum safety, git refresh generates a "recycle bin" directory
in your root and moves untracked files to said recycle bin instead of
actually deleting in the event that you need to recover any of these
files. refresh is configurable to stash (default) or restore any changes
to git-tracked files for maximum control. You can also exclude certain
files or directories from the untracked clean up by including them
in a ".gitrefresh" file in your directory to avoid removing key
files or directories (credentials, node_modules, etc).

### Why not git clean?
git clean is a built-in git command which allows you to delete
untracked files which even allows for exceptions via regex patterns.
git clean is an excellent git command and definitely useful in a pinch.

My goal for git refresh is not as a replacement for git clean but as an
alternate tool used for different situations. By allowing for stashes
and pulls to occur sequential to clean up, git refresh gets you closer
to the state of your remote repo all in one command. The recycle bin
allows for easy execution of cleanup without worrying if some important
files will be removed in the processs. And as per the below future
goals, git refresh aims to give you the capacity to execute git refresh
recursively from one starting directory down to all git-tracked
sub-directories in the event the user wants to regularly ensure that
all (or some subset) off their repos are up to date with git and don't
hide surprises in local files.

### Install
Currently, you can install this plugin for git by doing the following:
- git clone repo:
```bash
$ git clone https://github.com/dialectician4/go_git_refresh.git
```
- Find directory for git executable, `GIT_DIR`
- With cwd as this repo, install executable in the same directory as git itself:
```bash
$ GOBIN={GIT_DIR}; go install .
```
- Now you can run git refresh as the below:
```bash
$ git refresh .
```

### Roadmap
- Implemented
    - Basic git refresh functionality (untracked file cleanup,
    exception list, auto-stash and pull)
    - Configuration for pull actions (stash-and-pull,
    restore-and-pull, no-pull)
    - Disposed of files sent to root-path recycle bin automatically
    (recycle bin overwritten with each git refresh)
    - Pass -s as a no-delete option in the event you only want to
    update repos but not impact non-git files
    - pass -p or --propagate to find all git repos underneath
    the given directory and executes git-refresh on each with
    the same CLI configurations you initially passed
        - ex: If you have an all-encompassing projects/ directory
        and want to pull on all of them but not delete any files,
        you can go to projects/ and run git refresh -s -p 
        (propagates to subdirectories -p but does not delete files -s)
- Future Goals
    - Tighter integration with .gitignore - add your .gitrefresh
    exceptions to your .gitignore instead of as a separate file
        - Flag individual .gitignore entries using a # comment

### Notes
- This specific implementation of git refresh is built as a training
  exercise to learn Go. This project is std library-only/relies on
  no third party dependencies besides testing. A second version of
  git refresh would likely use a standard CLI-parsing library like 
  standard library's flag or pflag or cobra.
- Learning project highlights:
    - Uses goroutines for blazingly fast parallel execution across
      multiple repos with -p flag.
    - Uses channels to collect logs from across goroutines and
      sends them to a dedicated goroutine for writing to standard out
      safely.
    - Manually parses CLI flags without any further tools. More flexible
      than std/flag.
