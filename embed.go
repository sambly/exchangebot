package exchangebot

import "embed"

//go:embed frontend/dist/*
//go:embed frontend_v2/dist/*
var Content embed.FS
