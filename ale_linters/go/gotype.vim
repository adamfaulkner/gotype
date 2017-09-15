" Author: Adam Faulkner
" Description: Typecheck go files.

call ale#linter#Define('go', {
\   'name': 'gotype',
\   'output_stream': 'stderr',
\   'executable': 'gotype',
\   'command': 'gotype -origFilename %s -modifiedFilename %t',
\   'callback': 'ale_linters#go#gobuild#Handler',
\})
