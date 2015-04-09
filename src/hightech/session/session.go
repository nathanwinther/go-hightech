package session

import (
    "bytes"
    "fmt"
    "html/template"
    "net/http"
    "path/filepath"
    "strconv"
    "time"
    "github.com/nathanwinther/go-awsses"
    "github.com/nathanwinther/go-uuid4"
    "hightech/config"
    "hightech/dao"
    "hightech/logger"
)

type Session struct {
    Id int64
    Key string
}

var (
    SESSION_OFFSET = int64(60 * 60 * 24 * 14)
    VERIFY_OFFSET = int64(60 * 60 * 24 * 2)
    Templates *template.Template
)

func New() (*Session, error) {
    // Create new session
    skey, err := uuid4.New()
    if err != nil {
        return nil, err
    }

    q := `
        INSERT INTO user_session VALUES(
            NULL
            , ?
            , ?
            , ?
            , ?
        );
    `

    params := []interface{} {
        skey,
        time.Now().Unix() + SESSION_OFFSET,
        time.Now().Unix(),
        time.Now().Unix(),
    }

    result, err := dao.Exec(q, params)
    if err != nil {
        return nil, err
    }

    sid, err := result.LastInsertId()
    if err != nil {
        return nil, err
    }

    return &Session{
        Id: sid,
        Key: skey,
    }, nil
}

func Parse(r *http.Request) (*Session, error) {
    c, err := r.Cookie(config.Get("session_cookie_name"))
    if err != nil {
        return nil, err
    }

    q := `
        SELECT
            id
            , key
        FROM user_session s
        WHERE s.key = ?
        AND s.valid_until > ?;
    `

    params := []interface{} {
        c.Value,
        time.Now().Unix(),
    }

    s := new(Session)

    bind := []interface{} {
        &s.Id,
        &s.Key,
    }

    err = dao.Row(q, params, bind)
    if err != nil {
        return nil, err
    }

    return s, nil
}

func SendVerify() error {
    vkey, err := uuid4.New()
    if err != nil {
        return err
    }

    q := `
        INSERT INTO user_verify VALUES(
            NULL
            , ?
            , ?
            , ?
            , ?
        );
    `

    params := []interface{} {
        vkey,
        time.Now().Unix() + VERIFY_OFFSET,
        time.Now().Unix(),
        time.Now().Unix(),
    }

    _, err = dao.Exec(q, params)
    if err != nil {
        return err
    }

    url := fmt.Sprintf("%s/verify/%s", config.Get("baseurl"), vkey)

    loadTemplates()

    tpl := "verify"
    subject := "Please verify your High Tech account"

    html := new(bytes.Buffer)
    Templates.ExecuteTemplate(html, fmt.Sprintf("%s.html", tpl), url)

    text := new(bytes.Buffer)
    Templates.ExecuteTemplate(text, fmt.Sprintf("%s.txt", tpl), url)

    m := awsses.New(
        config.Get("awsses_sender"),
        config.Get("verify_email"),
        subject,
        html.String(),
        text.String())

    return m.Send(
        config.Get("awsses_baseurl"),
        config.Get("awsses_accesskey"),
        config.Get("awsses_secretkey"))
}

func Verify(vkey string) (*Session, error) {
    // Get id
    q := `
        SELECT
            id
        FROM user_verify
        WHERE key = ?
        AND valid_until > ?;
    `

    params := []interface{} {
        vkey,
        time.Now().Unix(),
    }

    var vid int64

    bind := []interface{} {
        &vid,
    }

    err := dao.Row(q, params, bind)
    if err != nil {
        return nil, err
    }

    // De-activate
    q = `
        UPDATE user_verify SET
            valid_until = 0
            , modified_date = ?
        WHERE id = ?;
    `

    params = []interface{} {
        time.Now().Unix(),
        vid,
    }

    _, err = dao.Exec(q, params)
    if err != nil {
        return nil, err
    }

    return New()
}

func (s *Session) Save(w http.ResponseWriter, keepalive bool) error {
    q := `
        UPDATE user_session SET
            valid_until = ?
            , modified_date = ?
        WHERE id = ?;
    `

    valid := int64(0)

    if keepalive {
        valid = time.Now().Unix() + SESSION_OFFSET
    }

    params := []interface{} {
        valid,
        time.Now().Unix(),
        s.Id,
    }

    _, err := dao.Exec(q, params)
    if err != nil {
        return err
    }

    expires := -1

    if keepalive {
        expires, _ = strconv.Atoi(config.Get("session_cookie_expires"))
    }

    secure, _ := strconv.ParseBool(config.Get("session_cookie_secure"))

    c := new(http.Cookie)
    c.Name = config.Get("session_cookie_name")
    if keepalive {
        c.Value = s.Key
    }
    c.Path = config.Get("session_cookie_path")
    c.MaxAge = expires
    c.Secure = secure
    
    http.SetCookie(w, c)
    logger.Log(w, "SET-COOKIE", c.String())
    return nil
}

func loadTemplates() {
    if Templates == nil {
        t, _ := template.ParseGlob(
            filepath.Join(config.Get("email"), "*.*"))
        Templates = t
    }
}

