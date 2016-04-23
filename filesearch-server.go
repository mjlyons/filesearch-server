package main

import (
  "encoding/json"
  "flag"
  "fmt"
  "github.com/mjlyons/filesearch"
  "log"
  "net/http"
  _ "net/http/pprof"  // Comment out to disable profiling
  "regexp"
  "time"
)

const PATH_WHITELIST string = "(py|js|coffee|go|yaml|scss|css|html)$"
const PATH_BLACKLIST string = "/(node_modules|build|coverage)/"

var searchableFiles [](*filesearch.FileData)

// TODO: Store this in a closure or something. Shouldn't be global
var srcRootPath *string
var workerCount *int
var buffering *int

func handleQuery(w http.ResponseWriter, r *http.Request) {
  query := r.URL.Query().Get("q")
  if query == "" {
    fmt.Fprintf(w, "Ya need a ?q=")
    w.WriteHeader(http.StatusBadRequest)
    return
  }

  fmt.Printf("Searching for %s...", query)
  startTime := time.Now()

  searchOptions := filesearch.SearchOptions{FilePathInclude: ""}
  searchResults, err := filesearch.SearchInDir(searchableFiles, *srcRootPath, query, &searchOptions, *workerCount, *buffering)
  if err != nil {
    log.Fatal(err)
  }

  w.Header().Set("Content-Type", "application/json; charset=UTF-8")

  searchTime := time.Since(startTime)
  json.NewEncoder(w).Encode(searchResults)

  //msecDelay := time.Since(startTime) / time.Millisecond
  totalTime := time.Since(startTime)
  fmt.Printf("Found %v files (%v latency, %v total)\n", len(searchResults), searchTime, totalTime)
}

// Loads each file's contents into the in-memory cache
func cacheAllFiles(fileDataSet [](*filesearch.FileData)) error {
  for _, fileData := range fileDataSet {
    _, err := fileData.GetContents(true, false)
    if err != nil {
      return err
    }
  }
  return nil
}

func main() {
  // Profiling
  go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()

  fmt.Println("Building file list...")

  startupStartTime := time.Now()

  precacheAllFiles := flag.Bool("precache-all-files", false,
    "Loads all file contents into memory to speed up searches")
  srcRootPath = flag.String("src-root", "", "Root path of source to search")
  workerCount = flag.Int("worker-count", 1, "Number of search workers")
  buffering = flag.Int("buffering", 10, "How much buffering between workers and feeder")
  perfTest := flag.Bool("perf-test", false, "Run a search in a loop for perf testing")
  flag.Parse()

  var filePathIncludeRegexp, filePathExcludeRegexp *regexp.Regexp
  var err error
  filePathIncludeRegexp, err = regexp.Compile(PATH_WHITELIST)
  if err != nil {
    log.Fatal(err)
  }
  filePathExcludeRegexp, err = regexp.Compile(PATH_BLACKLIST)
  if err != nil {
    log.Fatal(err)
  }

  searchableFiles, err = filesearch.GetFilepathsInDir(*srcRootPath, filePathIncludeRegexp, filePathExcludeRegexp)
  if err != nil {
    log.Fatal(err)
  }

  if (*precacheAllFiles) {
    fmt.Println("Caching file contents...")
    err := cacheAllFiles(searchableFiles)
    if err != nil {
      log.Fatal(err)
    }
  }

  startupDuration := time.Since(startupStartTime)
  log.Printf("Listening... (startup took %v)\n", startupDuration)

  if *perfTest {
    for {
      queryStartTime := time.Now()
      query := "PdfLoader"
      searchOptions := filesearch.SearchOptions{FilePathInclude: PATH_WHITELIST}
      searchResults, err := filesearch.SearchInDir(searchableFiles, *srcRootPath, query, &searchOptions, *workerCount, *buffering)
      if err != nil {
        log.Fatal(err)
      }
      fmt.Println(len(searchResults), time.Since(queryStartTime))
    }
    log.Fatal("How'd you get out of the perf test?")
  }

  http.HandleFunc("/search", handleQuery)
  log.Fatal(http.ListenAndServe(":8080", nil))

}
