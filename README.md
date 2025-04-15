git refresh
------------
git refresh is a git plugin which allows you to simultaneously remove
untracked files from your repository and pull from git to help
synchronize the state of your repo with remote.

### Why git refresh?
Have you ever worked on a project where one process or another generates
a variety of cache files for some purpose or another? And you leave
these files be until some result starts differing between 2 peoples'
machines and someone says "It works on my machine?" I've seen this
happen and the issue persists until someone deletes their repo altogether
and reclones and the issue is revealed to lie in caches or other non-git
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
to tracked files for maximum control. You can also exclude certain
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
- Future Goals
    - Tighter integration with .gitignore - add your .gitrefresh
    exceptions to your .gitignore instead of as a separate file
        - Flag individual .gitignore entries using a # comment
    - Add option for recursively executing git refresh on git-tracked
    repos nested below the given path





