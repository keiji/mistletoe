// Package app implements the core application logic.
package app

import (
	conf "mistletoe/internal/config"
)

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)
