package configparser

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var absPath string
var data map[string][]byte
var forceType = "auto"
var glue string
var useArrayAsObject bool

func Parse(path string, dataDest map[string][]byte, arrayGlue string, arrayAsObject bool) error {
	absPath, _ = filepath.Abs(path)
	data = dataDest
	glue = arrayGlue
	useArrayAsObject = arrayAsObject
	_, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if forceType == "tar" || strings.HasSuffix(strings.ToLower(path), ".tar") {
		fp, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fp.Close()
		tarReader := tar.NewReader(fp)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			// a tar can have some annoying paths
			path := strings.TrimPrefix(header.Name, "./")
			info := header.FileInfo()
			err = fpWalk(path, info, tarReader, nil)
			if err != nil && err != filepath.SkipDir {
				return err
			}
		}
	} else {
		err = filepath.Walk(absPath, walk)
	}
	return err
}

func ParseAsJSON(path string, dataDest map[string][]byte, arrayGlue string, arrayAsObject bool) error {
	forceType = "json"
	return Parse(path, dataDest, arrayGlue, arrayAsObject)
}

func ParseAsYAML(path string, dataDest map[string][]byte, arrayGlue string, arrayAsObject bool) error {
	forceType = "yaml"
	return Parse(path, dataDest, arrayGlue, arrayAsObject)
}

func ParseAsTAR(path string, dataDest map[string][]byte, arrayGlue string, arrayAsObject bool) error {
	forceType = "tar"
	return Parse(path, dataDest, arrayGlue, arrayAsObject)
}

func walk(path string, fstat os.FileInfo, err error) error {
	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()
	return fpWalk(path, fstat, fp, err)
}
func fpWalk(path string, fstat os.FileInfo, fp io.Reader, err error) error {
	// skip .git etc
	if fstat.IsDir() && strings.HasPrefix(fstat.Name(), ".") {
		return filepath.SkipDir
	}
	if fstat.Mode().IsDir() {
		return nil
	}
	keyPrefix := strings.Split(
		// remove leading '/'
		strings.TrimPrefix(
			// remove the file extension
			strings.TrimSuffix(
				// remove the base path that was passed as -path
				strings.TrimPrefix(path, absPath),
				filepath.Ext(path)),
			string(os.PathSeparator)),
		string(os.PathSeparator))
	if keyPrefix[0] == "" {
		// remove blank token
		keyPrefix = []string{}
	}
	switch {
	// skip dotfiles
	case strings.HasPrefix(fstat.Name(), "."):
		return nil
	case strings.HasSuffix(strings.ToLower(path), ".json") || forceType == "json":
		err := parseJson(fp, keyPrefix, glue)
		if err != nil {
			return err
		}
	case strings.HasSuffix(strings.ToLower(path), ".yml"):
		fallthrough
	case strings.HasSuffix(strings.ToLower(path), ".yaml") || forceType == "yaml":
		// yaml handling based on https://github.com/bronze1man/yaml2json
		yamlR, yamlW := io.Pipe()
		go func() {
			defer yamlW.Close()
			yamlToJson(fp, yamlW)
		}()
		err := parseJson(yamlR, keyPrefix, glue)
		if err != nil {
			return err
		}
	//case strings.HasSuffix(strings.ToLower(path), ".properties"):
	// TODO: .properties parsing
	//case strings.HasSuffix(strings.ToLower(path), ".ini"):
	// TODO: .ini parsing
	// filenames with no type, or .txt should be handled as raw data
	case !strings.Contains(fstat.Name(), ".") || strings.HasSuffix(strings.ToLower(path), ".txt"):
		err := parseRaw(fp, keyPrefix, glue)
		if err != nil {
			return err
		}
	default:
	}
	return nil
}
