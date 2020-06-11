package filters

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testStartFilterListener() net.Listener {
	http.HandleFunc("/filters/1.txt", func(w http.ResponseWriter, r *http.Request) {
		content := `||example.org^$third-party
# Inline comment example
||example.com^$third-party
0.0.0.0 example.com
`
		_, _ = w.Write([]byte(content))
	})
	http.HandleFunc("/filters/2.txt", func(w http.ResponseWriter, r *http.Request) {
		content := `||example.org^$third-party
# Inline comment example
||example.com^$third-party
0.0.0.0 example.com
1.1.1.1 example1.com
`
		_, _ = w.Write([]byte(content))
	})

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	go func() {
		_ = http.Serve(listener, nil)
	}()
	return listener
}

func prepareTestDir() string {
	const dir = "./agh-test"
	_ = os.RemoveAll(dir)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func TestFilters(t *testing.T) {
	lhttp := testStartFilterListener()
	defer func() { _ = lhttp.Close() }()

	dir := prepareTestDir()
	defer func() { _ = os.RemoveAll(dir) }()

	fconf := Conf{}
	fconf.FilterDir = dir
	fconf.HTTPClient = &http.Client{
		Timeout: 5 * time.Second,
	}
	fs := New(fconf)
	// fs.Start()

	port := lhttp.Addr().(*net.TCPAddr).Port
	URL := fmt.Sprintf("http://127.0.0.1:%d/filters/1.txt", port)

	// add and download
	f := Filter{
		URL: URL,
	}
	err := fs.Add(f)
	assert.Equal(t, nil, err)

	// check
	l := fs.List(0)
	assert.Equal(t, 1, len(l))
	assert.Equal(t, URL, l[0].URL)
	assert.True(t, l[0].Enabled)
	assert.Equal(t, uint64(3), l[0].RuleCount)
	assert.True(t, l[0].ID != 0)

	// disable
	st, _, err := fs.Modify(f.URL, false, "name", f.URL)
	assert.Equal(t, StatusChangedEnabled, st)

	// check: disabled
	l = fs.List(0)
	assert.Equal(t, 1, len(l))
	assert.True(t, !l[0].Enabled)

	// modify URL
	newURL := fmt.Sprintf("http://127.0.0.1:%d/filters/2.txt", port)
	st, modified, err := fs.Modify(URL, false, "name", newURL)
	assert.Equal(t, StatusChangedURL, st)

	_ = os.Remove(modified.Path)

	// check: new ID, new URL
	l = fs.List(0)
	assert.Equal(t, 1, len(l))
	assert.Equal(t, newURL, l[0].URL)
	assert.Equal(t, uint64(4), l[0].RuleCount)
	assert.True(t, modified.ID != l[0].ID)

	removed := fs.Delete(newURL)
	assert.NotNil(t, removed)
	_ = os.Remove(removed.Path)

	fs.Close()
}