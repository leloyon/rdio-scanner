// Update check (custom Lel edition).
//
// The admin panel asks GET /api/admin/update-check whether a newer release has been published on
// GitHub. The running exe hashes itself (SHA-256) and looks for that hash among the release
// assets' digests, which tells which release it came from, with no version labels to maintain:
//
//	newest release matches    -> "current"
//	an older release matches  -> "available" (details of the newest release are returned)
//	no release matches        -> "unknown"   (e.g. a local build that was not uploaded yet)
//
// GitHub is only contacted when an admin opens the panel, and results are cached for 30 minutes.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	updateRepo     = "leloyon/rdio-scanner"
	updateCacheTTL = 30 * time.Minute
	updateErrorTTL = 5 * time.Minute
)

type ghAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	URL    string `json:"browser_download_url"`
}

type ghRelease struct {
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Published  string    `json:"published_at"`
	HTMLURL    string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

type updateRelease struct {
	Tag       string `json:"tag"`
	Name      string `json:"name"`
	Published string `json:"published"`
	URL       string `json:"url"`
	AssetName string `json:"assetName,omitempty"`
	AssetURL  string `json:"assetUrl,omitempty"`
	AssetSize int64  `json:"assetSize,omitempty"`
}

type updateInfo struct {
	Version string         `json:"version,omitempty"` // base Rdio Scanner version of the running server
	Status  string         `json:"status"`            // current, available, unknown or error
	Current *updateRelease `json:"current,omitempty"`
	Latest  *updateRelease `json:"latest,omitempty"`
}

func toUpdateRelease(r ghRelease) *updateRelease {
	u := &updateRelease{Tag: r.Tag, Name: r.Name, Published: r.Published, URL: r.HTMLURL}
	if u.Name == "" {
		u.Name = r.Tag
	}
	var pick *ghAsset
	for i := range r.Assets {
		if strings.HasSuffix(strings.ToLower(r.Assets[i].Name), ".exe") {
			pick = &r.Assets[i]
			break
		}
	}
	if pick == nil && len(r.Assets) > 0 {
		pick = &r.Assets[0]
	}
	if pick != nil {
		u.AssetName, u.AssetURL, u.AssetSize = pick.Name, pick.URL, pick.Size
	}
	return u
}

// evaluateUpdate decides the update status from the published releases and the running exe's SHA-256.
func evaluateUpdate(releases []ghRelease, ownHash string) updateInfo {
	list := []ghRelease{}
	for _, r := range releases {
		if !r.Draft && !r.Prerelease {
			list = append(list, r)
		}
	}
	if len(list) == 0 || ownHash == "" {
		return updateInfo{Status: "unknown"}
	}

	sort.SliceStable(list, func(i, j int) bool { return list[i].Published > list[j].Published })

	want := "sha256:" + strings.ToLower(ownHash)
	mine := -1
	for i, r := range list {
		for _, a := range r.Assets {
			if strings.ToLower(a.Digest) == want {
				mine = i
				break
			}
		}
		if mine >= 0 {
			break
		}
	}

	switch {
	case mine < 0:
		return updateInfo{Status: "unknown"}
	case mine == 0:
		return updateInfo{Status: "current", Current: toUpdateRelease(list[0])}
	default:
		return updateInfo{Status: "available", Current: toUpdateRelease(list[mine]), Latest: toUpdateRelease(list[0])}
	}
}

var (
	updateOnce  sync.Once
	updateHash  string
	updateMutex sync.Mutex
	updateAt    time.Time
	updateLast  updateInfo
)

func ownExeHash() string {
	updateOnce.Do(func() {
		path, err := os.Executable()
		if err != nil {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return
		}
		updateHash = hex.EncodeToString(h.Sum(nil))
	})
	return updateHash
}

func fetchReleases() ([]ghRelease, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+updateRepo+"/releases?per_page=20", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rdio-scanner-update-check")

	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned %s", res.Status)
	}

	var releases []ghRelease
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func checkForUpdate() updateInfo {
	updateMutex.Lock()
	defer updateMutex.Unlock()

	ttl := updateCacheTTL
	if updateLast.Status == "error" {
		ttl = updateErrorTTL
	}
	if updateLast.Status != "" && time.Since(updateAt) < ttl {
		return updateLast
	}

	releases, err := fetchReleases()
	if err != nil {
		updateLast = updateInfo{Status: "error"}
	} else {
		updateLast = evaluateUpdate(releases, ownExeHash())
	}
	updateAt = time.Now()

	return updateLast
}

// UpdateCheckHandler serves GET /api/admin/update-check (admin only).
func (admin *Admin) UpdateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if !admin.ValidateToken(admin.GetAuthorization(r)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	info := checkForUpdate()
	info.Version = Version

	b, err := json.Marshal(info)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
