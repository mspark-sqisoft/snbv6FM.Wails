//go:build production && !snb_noenforce

package config

func init() { sealed = true }
