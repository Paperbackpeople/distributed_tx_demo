package infra

import "github.com/google/uuid"

func NewGID() string { return uuid.New().String() }
