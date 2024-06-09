package main

import (
    "errors"
    "flag"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "strconv"
    "strings"
)

func parseIntOption(request *http.Request, key string) (int, error) {
    if value := request.URL.Query().Get(key); value != "" {
        valueStrConv, err := strconv.Atoi(value)
        if err == nil {
            return valueStrConv, nil
        } else {
            return -1, errors.New("invalid integer option value provided")
        }
    }
    return -1, nil
}

func parseBoolOption(request *http.Request, key string) bool {
    value := request.URL.Query().Get(key)
    return value == "1"
}

type pdfOpts struct {
    grayscale    bool
    lowquality   bool
    orientation  string
    forms        bool
    images       bool
    javascript   bool
    pagesize     string
    title        string
    imagedpi     *int
    imagequality *int
    marginleft   string
    marginright  string
    margintop    string
    marginbottom string
    shrinking    bool
}

func parsePdfOptions(request *http.Request) (pdfOpts, error) {
    opts := pdfOpts{}

    opts.grayscale = parseBoolOption(request, "grayscale")
    opts.lowquality = parseBoolOption(request, "lowquality")
    opts.forms = parseBoolOption(request, "forms")
    opts.images = !parseBoolOption(request, "noimages")
    opts.javascript = !parseBoolOption(request, "nojavascript")
    opts.shrinking = !parseBoolOption(request, "shrinking")
    opts.marginleft = request.URL.Query().Get("marginleft")
    opts.marginright = request.URL.Query().Get("marginright")
    opts.margintop = request.URL.Query().Get("margintop")
    opts.marginbottom = request.URL.Query().Get("marginbottom")
    if imagedpi, err := parseIntOption(request, "imagedpi"); imagedpi > 0 {
        opts.imagedpi = &imagedpi
    } else if err != nil {
        return opts, errors.New("invalid imagedpi value provided")
    }
    if imagequality, err := parseIntOption(request, "imagequality"); imagequality > 0 {
        opts.imagequality = &imagequality
    } else if err != nil {
        return opts, errors.New("invalid imagequality value provided")
    }

    orientation := request.URL.Query().Get("orientation")
    if orientation == "P" {
        opts.orientation = "Portrait"
    } else if orientation == "L" {
        opts.orientation = "Landscape"
    } else if orientation == "" {
        opts.orientation = "Portrait"
    } else {
        return opts, errors.New("invalid orientation value provided")
    }

    opts.pagesize = request.URL.Query().Get("pagesize")
    if opts.pagesize == "" {
        opts.pagesize = "A4"
    }

    opts.title = request.URL.Query().Get("title")

    return opts, nil
}

func preparePdfArgs(opts pdfOpts) []string {
    args := []string{"--encoding", "utf-8"}
    if opts.grayscale {
        args = append(args, "--grayscale")
    }
    if opts.marginleft != "" {
        args = append(args, "--margin-left", opts.marginleft)
    }
    if opts.marginright != "" {
        args = append(args, "--margin-right", opts.marginright)
    }
    if opts.margintop != "" {
        args = append(args, "--margin-top", opts.margintop)
    }
    if opts.marginbottom != "" {
        args = append(args, "--margin-bottom", opts.marginbottom)
    }
    if opts.lowquality {
        args = append(args, "--lowquality")
    }
    if opts.forms {
        args = append(args, "--enable-forms")
    }
    if !opts.images {
        args = append(args, "--no-images")
    }
    if !opts.shrinking {
        args = append(args, "--disable-smart-shrinking")
    }
    if !opts.javascript {
        args = append(args, "--disable-javascript")
    }
    args = append(args, "--orientation", opts.orientation)
    args = append(args, "--page-size", opts.pagesize)
    if opts.title != "" {
        args = append(args, "--title", opts.title)
    }
    if opts.imagedpi != nil {
        args = append(args, "--image-dpi", strconv.Itoa(*opts.imagedpi))
    }
    if opts.imagequality != nil {
        args = append(args, "--image-quality", strconv.Itoa(*opts.imagequality))
    }
    args = append(args, "--include-in-outline")
    args = append(args, "-", "-")
    return args
}

func runWkhtmltopdf(response http.ResponseWriter, html string, args []string) {
    binPath := os.Getenv("WKHTMLTOPDF_PATH")
    if binPath == "" {
        binPath = "wkhtmltopdf"
    }
    cmd := exec.Command(binPath, args...)
    stdin, err := cmd.StdinPipe()
    if err != nil {
        response.WriteHeader(500)
        log.Println("An error occurred:", err)
        return
    }
    defer stdin.Close()
    cmd.Stdout = response

    log.Printf("Rendering PDF. Arguments: '%s' ... ", strings.Join(args, " "))
    if err = cmd.Start(); err != nil {
        response.WriteHeader(500)
        log.Println("An error occurred:", err)
        return
    }

    response.Header().Set("Content-Type", "application/pdf")
    stdin.Write([]byte(html))
    stdin.Close()
    cmd.Wait()
    log.Println("done")
}

func handlePdf(response http.ResponseWriter, request *http.Request) {
    if request.Method == "OPTIONS" {
        response.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
        response.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        response.WriteHeader(200)
        return
    }
    
    if request.Method != "POST" {
        response.WriteHeader(405)
        response.Header().Set("Allow", "POST")
        return
    }

    var html string
    var err error

    // Check for file upload
    file, _, err := request.FormFile("htmlfile")
    if err == nil {
        defer file.Close()
        bhtml, err := ioutil.ReadAll(file)
        if err != nil {
            response.WriteHeader(400)
            return
        }
        html = string(bhtml)
    } else {
        // Otherwise, check for HTML string
        bhtml, err := ioutil.ReadAll(request.Body)
        if err != nil {
            response.WriteHeader(400)
            return
        }
        html = string(bhtml)
    }

    if html == "" {
        response.WriteHeader(400)
        return
    }

    opts, err := parsePdfOptions(request)
    if err != nil {
        response.WriteHeader(400)
        response.Write([]byte(err.Error()))
        return
    }
    args := preparePdfArgs(opts)
    runWkhtmltopdf(response, html, args)
}

func main() {
    var portNumber int
    flag.IntVar(&portNumber, "port", 0, "Port to listen on")
    flag.Parse()
    if portNumber == 0 {
        portNumberStr := os.Getenv("WKHTMLTOX_PORT")
        if portNumberStr == "" {
            portNumber = 8080
        } else {
            portNumberStrConv, err := strconv.Atoi(portNumberStr)
            if err == nil {
                portNumber = portNumberStrConv
            } else {
                log.Println(err)
                os.Exit(2)
            }
        }
    }

    http.HandleFunc("/pdf", handlePdf)
    log.Fatal(http.ListenAndServe(":"+strconv.Itoa(portNumber), nil))
}
