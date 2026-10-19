package vm

import "testing"

// ssh runs ProxyCommand through /bin/sh on Unix and as a plain process on Windows, and expands % tokens on both.
func TestQuote_PerOperatingSystem(t *testing.T) {
	cases := []struct {
		goos string
		path string
		want string
	}{
		{"linux", "/usr/local/bin/openvicloud", "'/usr/local/bin/openvicloud'"},
		{"darwin", "/opt/open vicloud/openvicloud", "'/opt/open vicloud/openvicloud'"},
		{"linux", "/Users/R&D/bin/openvicloud", "'/Users/R&D/bin/openvicloud'"},
		{"linux", "/home/o'neil/bin/openvicloud", `'/home/o'\''neil/bin/openvicloud'`},
		{"linux", "/home/a%b/openvicloud", "'/home/a%%b/openvicloud'"},
		{"windows", `C:\Users\me\openvicloud.exe`, `"C:\Users\me\openvicloud.exe"`},
		{"windows", `C:\Program Files\OpenViCloud\openvicloud.exe`, `"C:\Program Files\OpenViCloud\openvicloud.exe"`},
		{"windows", `C:\Users\R&D\openvicloud.exe`, `"C:\Users\R&D\openvicloud.exe"`},
		{"windows", `C:\Users\a%b\openvicloud.exe`, `"C:\Users\a%%b\openvicloud.exe"`},
	}
	for _, c := range cases {
		got := quoteFor(c.goos, c.path)
		if got != c.want {
			t.Errorf("quoteFor(%q, %q) = %q, want %q", c.goos, c.path, got, c.want)
		}
	}
}
