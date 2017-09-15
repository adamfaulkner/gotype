Hacked Together GoType
=================

What this is
-----------

I took [gotype](https://godoc.org/golang.org/x/tools/cmd/gotype) from the master
branch of Go's git repo and changed a few lines around to make it build in go
1.8.

I added the srcimporter from Go's master branch as well.

I added two flags to it (-origFilename and -modifiedFilename) to enable
typechecking as you type in my editor.

I added a plugin for [ALE](https://github.com/w0rp/ale) to allow syntax checking
in vim.


Why I made this
---------------

I used to use go build for my syntax checking needs. It had several issues:

- I often work in an environment where I cannot build what I'm working on
  locally due to cgo dependencies. This prevents me from using go build in most
  packages I work on.

- I find go build to be extremely resource intensive after doing a git pull or
  changing branches as it has to rebuild lots of dependencies.

- Go build only works after a file is saved. So my workflow involved a lot of
  unnecessary saving to see if my program typechecked.


This hacked up go type solves these problems:

- It uses the srcimporter rather than trying to compile packages, so packages
  that depend on C libraries can still be typechecked. Unfortunately, cgo
  bindings themselves cannot be typechecked.

- It is faster than go build.

- I've added arguments to support as-you-type typechecking.


Usage: Vim
-----------
- Download this project.

- Build gotype and put it somewhere on your PATH:
	```
	go build .
	cp gotype ~/bin/
	```

- Install [ALE](https://github.com/w0rp/ale)

- Put this project's root directory on your vim runtimepath. (I use
  [Plug](https://github.com/junegunn/vim-plug) for this). In your vimrc:
	```
	call plug#begin('~/.vim/plugged')
	...
	Plug 'w0rp/ale'
	Plug '~/Downloads/gotype/'
	...
	call plug#end()
	```

- Configure ALE to use gotype on go files. In your vimrc:

	```
	let g:ale_linters = {
	\   'go': ['gotype'],
	\}
	```

- Try it out!

Usage: Emacs
-----------
I had a plugin that worked with emacs [flycheck](http://www.flycheck.org/), but
I've stopped maintaining it. It would be pretty trivial to get it working
though.

Caveats
-------
- Doesn't work directly on cgo code, returns spurious errors.
- Still fairly slow.


TODO
----
- cgo is not correctly handled.
- Implement checker as a persistant service that can cache unmodified packages.
  Experiments show a 2x performance improvement.
