package tests

import (
	"path"
	"runtime"
	"sync"
)

// AppWorkDir - возвращает рабочую директорию приложения.
func AppWorkDir() string {
	var (
		once    sync.Once
		workDir string
	)

	once.Do(func() {
		_, p, _, _ := runtime.Caller(0)
		workDir = path.Join(path.Dir(p), "..") // up from '.../tests'
	})

	return workDir
}

// DBSchemas - возвращает массив схем БД, с которыми работает приложение.
func DBSchemas() []string {
	return []string{
		"sample_schema",
	}
}

// ExcludedDBTables - возвращает массив таблиц БД, которые не должны меняться при тестировании.
func ExcludedDBTables() []string {
	return nil
}
