package main

import "os"

// isDirExists checks if the directory exists or not and returns it.
func isDirExists(flags map[string]any) string {
	dir, ok := flags["directory"].(string)
	if !ok {
		return ""
	}
	// Check if the directory exists or not
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}

	return dir
}
