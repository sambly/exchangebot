package main

import "embed"

//go:embed frontend/dist/*
var Content embed.FS
