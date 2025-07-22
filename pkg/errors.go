package pkg

import "errors"

var ErrResourceNotFound = errors.New("resource not found")
var ErrResourceMetadataNotFound = errors.New("resource metadata not found")
var ErrProjectNotFound = errors.New("project not found")
