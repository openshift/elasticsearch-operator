#!/bin/bash

cd $(dirname "${BASH_SOURCE[0]}")

# to use it in the best way, pipe output to `pbcopy` to save it in the paste buffer
# on the other end, run the command `echo > dest_script.sh && vi dest_script.sh`
# in it paste the data from the paste buffer and save

cat ../pkg/indexmanagement/scripts.go | \
	 awk 'NR>90' | \
	 sed '$d' | \
	 sed '$d'| \
	 sed '$d'| \
	 sed '$d'| \
	 sed '$d'| \
	 sed '$d'| \
	 sed '$d'
