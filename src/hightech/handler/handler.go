package handler

import (
    "bytes"
    "fmt"
    "encoding/json"
    "html/template"
    "io/ioutil"
    "net/http"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
    "github.com/nathanwinther/go-awsses"
    "github.com/nathanwinther/go-uuid4"
    "hightech/config"
    "hightech/flashdata"
    "hightech/invoice"
    "hightech/logger"
    "hightech/session"
)

type Handler struct {
    Rules []*Rule
    Templates *template.Template
    header string
}

type Rule struct {
    Pattern string
    Compiled *regexp.Regexp
    Handler func(http.ResponseWriter, *http.Request)
}

func New() (*Handler, error) {
    h := new(Handler)

    h.header = config.Get("response_header")

    h.Rules = []*Rule {
        &Rule{"GET:/hightech", nil, h.handleHome},
        &Rule{"GET:/hightech/[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]", nil, h.handleHome},
        &Rule{"GET:/hightech/pull", nil, h.handlePull},
        &Rule{"GET:/hightech/purge", nil, h.handlePurge},
        &Rule{"GET:/hightech/verify/[A-Za-z0-9][A-Za-z0-9-]*", nil, h.handleVerify},
        &Rule{"POST:/hightech/close", nil, h.handleClose},
        &Rule{"POST:/hightech/update", nil, h.handleUpdate},
        &Rule{"POST:/hightech/verify", nil, h.handleVerifyPost},
    }

    // Compile rules
    for _, rule := range h.Rules {
        re, err := regexp.Compile(fmt.Sprintf("^%s$",
            strings.TrimRight(rule.Pattern, "/")))
        if err != nil {
            panic(err)
        }
        rule.Compiled = re
    }

    err := h.loadTemplates()
    if err != nil {
        return nil, err
    }

    return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    b := []byte(fmt.Sprintf("%s:/%s", r.Method, strings.Trim(r.URL.Path, "/")))

    rid, _ := uuid4.New()
    w.Header().Add(h.header, rid)

    logger.Log(w, "INCOMING", string(b))

    for _, rule := range h.Rules {
        if rule.Compiled.Match(b) {
            rule.Handler(w, r)
            return
        }
    }

    h.serveNotFound(w, r)
}

func (h *Handler) handleClose(w http.ResponseWriter, r *http.Request) {
    _, err := session.Parse(r)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    path := config.Get("invoice_data")
    v, err := invoice.Load(path)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    // HTML
    htmlName := fmt.Sprintf("%s-%s.html", v.User.Prefix,
        v.Invoice.Entries[v.Invoice.EndDate].Key)
    htmlPath := filepath.Join(config.Get("archive_path"), htmlName)

    var htmlBuf bytes.Buffer
    err = h.Templates.ExecuteTemplate(&htmlBuf, "invoice.html", v)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    err = ioutil.WriteFile(htmlPath, htmlBuf.Bytes(), 0644)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    // JSON
    jsonName := fmt.Sprintf("%s-%s.txt", v.User.Prefix,
        v.Invoice.Entries[v.Invoice.EndDate].Key)
    jsonPath := filepath.Join(config.Get("archive_path"), jsonName)

    jsonBytes, err := json.MarshalIndent(v, "", "    ")
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    err = ioutil.WriteFile(jsonPath, jsonBytes, 0644)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    // EMAIL
    m := awsses.New(
        config.Get("awsses_sender"),
        config.Get("awsses_sender"),
        fmt.Sprintf(
            "High Tech Timesheet %s/%s/%d",
            v.Invoice.Entries[v.Invoice.EndDate].MM,
            v.Invoice.Entries[v.Invoice.EndDate].DD,
            v.Invoice.Entries[v.Invoice.EndDate].YYYY),
        "",
        fmt.Sprintf("%d Hours", v.Invoice.Total),
        &awsses.MessageAttachment{htmlBuf.Bytes(), "text/html", htmlName},
        &awsses.MessageAttachment{jsonBytes, "text/plain", jsonName})

    err = m.Send(
        config.Get("awsses_baseurl"),
        config.Get("awsses_accesskey"),
        config.Get("awsses_secretkey"))
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    v2, err := invoice.New(v.Invoice.Entries[v.Invoice.EndDate].Key, time.Hour * 24)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    v2.User = v.User
    v2.User.LastInvoice = fmt.Sprintf("%s/%s", 
        config.Get("archive_baseurl"), htmlName)

    err = v2.Save(path)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    http.Redirect(w, r, config.Get("baseurl"), http.StatusFound)
}

func (h *Handler) handleHome(w http.ResponseWriter, r *http.Request) {
    s, _ := session.Parse(r)
    if s != nil {
        s.Save(w, true)
    }

    v, err := invoice.Load(config.Get("invoice_data"))
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    urlpath := strings.Trim(r.URL.Path, "/")
    segments := strings.Split(urlpath, "/")

    if len(segments) == 2 {
        v.SetSelected(segments[1])
    }

    msg, _ := flashdata.Get(w, r)

    m := map[string] interface{} {
        "Hours": make([]int, 25),
        "Invoice": v,
        "LoggedIn": s != nil,
        "Message": msg,
        "Url": r.URL.Path,
    }

    h.Templates.ExecuteTemplate(w, "home.html", m)
}

func (h *Handler) handlePull(w http.ResponseWriter, r *http.Request) {
    v, err := invoice.Load(config.Get("invoice_data"))
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    m := new(invoice.Migrate)
    m.Invoice = v

    match, err := filepath.Glob(
        filepath.Join(config.Get("archive_path"), "?*.???*"))
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    m.Archive = make([]*invoice.MigrateArchive, len(match))

    for i, f := range match {
        f = filepath.Base(f)
        m.Archive[i] = &invoice.MigrateArchive{
            f,
            fmt.Sprintf("%s%s/%s", config.Get("baseurl"), 
                config.Get("archive_baseurl"), f)}
    }

    b, err := json.MarshalIndent(m, "", "    ")
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    w.Header().Add("Content-Type", "text/plain")
    w.Write(b)
}

func (h *Handler) handlePurge(w http.ResponseWriter, r *http.Request) {
    err := h.loadTemplates()
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    w.Header().Add("Content-Type", "text/plain")
    w.Write([]byte("Templates Reloaded"))
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
    _, err := session.Parse(r)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    path := config.Get("invoice_data")
    v, err := invoice.Load(path)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    key := r.FormValue("key")
    hours, _ := strconv.Atoi(r.FormValue("hours"))

    v.SetHours(key, hours)
    err = v.Save(path)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    url := r.FormValue("url")
    if url == "" {
        url = config.Get("baseurl")
    }

    http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) handleVerify(w http.ResponseWriter, r *http.Request) {
    segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    vkey := segments[2]

    s, err := session.Verify(vkey)
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    s.Save(w, true)

    http.Redirect(w, r, config.Get("baseurl"), http.StatusFound)
}

func (h *Handler) handleVerifyPost(w http.ResponseWriter, r *http.Request) {
    err := session.SendVerify()
    if err != nil {
        logger.Error(w, err)
        h.serveServerError(w, r)
        return
    }

    flashdata.Set(w, "Verification link sent to your email address")

    http.Redirect(w, r, config.Get("baseurl"), http.StatusFound)
}

func (h *Handler) loadTemplates() error {
    t, err := template.ParseGlob(filepath.Join(config.Get("templates"), "*.*"))
    if err != nil {
        return err
    }

    h.Templates = t

    return nil
}

func (h *Handler) serveNotFound(w http.ResponseWriter, r *http.Request) {
    m := map[string] interface{} {
        "baseurl": config.Get("baseurl"),
    }
    w.WriteHeader(http.StatusNotFound)
    h.Templates.ExecuteTemplate(w, "error404.html", m)
}

func (h *Handler) serveServerError(w http.ResponseWriter, r *http.Request) {
    m := map[string] interface{} {
        "baseurl": config.Get("baseurl"),
    }
    w.WriteHeader(http.StatusInternalServerError)
    h.Templates.ExecuteTemplate(w, "error500.html", m)
}

