package main

import "os"

func ownedByRoot(os.FileInfo) bool { return false }
