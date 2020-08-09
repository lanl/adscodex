#!/bin/bash

for i in *.data ; do
	name=`echo $i | sed 's/\..*//'`
	for p in *.gpl ; do
		gnuplot -e name=\"$name\" $p
	done
done
