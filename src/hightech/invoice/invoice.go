package invoice

import (
    "encoding/json"
    "io/ioutil"
    "strconv"
    "time"
)

type Invoice struct {
    User User
    Invoice InvoiceTimesheet
}

type InvoiceTimesheet struct {
    StartDate int
    EndDate int
    Selected int
    SelectedInRange bool
    Entries []InvoiceTimesheetEntry
    Total int
    Version int
}

type InvoiceTimesheetEntry struct {
    Key string
    Hours int
    Selected bool
    Today bool
    D int
    DD string
    DDD string
    DDDD string
    M int
    MM string
    MMM string
    MMMM string
    YY string
    YYYY int
}

type User struct {
    Name string
    Company string
    Supervisor string
    Prefix string
    LastInvoice string
}

type Migrate struct {
    Invoice *Invoice
    Archive []*MigrateArchive
}

type MigrateArchive struct {
    Name string
    Url string
}

func Load(path string) (*Invoice, error) {
    b, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }

    v := new(Invoice)

    err = json.Unmarshal(b, v)
    if err != nil {
        return nil, err
    }

    v.SetSelected("")
    v.SetTotal()

    return v, nil
}

func New(key string, offset time.Duration) (*Invoice, error) {
    t, err := time.Parse("2006-01-02", key)
    if err != nil {
        return nil, err
    }

    t = t.Add(offset)

    v := new(Invoice)
    v.User = *new(User)
    v.Invoice = *new(InvoiceTimesheet)
    v.Invoice.Entries = make([]InvoiceTimesheetEntry, 14)

    for i := 0; i < 14; i++ {
        e := new(InvoiceTimesheetEntry)
        e.Key = t.Format("2006-01-02")
        e.D, _ = strconv.Atoi(t.Format("02"))
        e.DD = t.Format("02")
        e.DDD = t.Format("Mon")
        e.DDDD = t.Format("Monday")
        e.M, _ = strconv.Atoi(t.Format("01"))
        e.MM = t.Format("01")
        e.MMM = t.Format("Jan")
        e.MMMM = t.Format("January")
        e.YY = t.Format("06")
        e.YYYY, _ = strconv.Atoi(t.Format("2006"))

        v.Invoice.Entries[i] = *e

        t = t.Add(time.Hour * 24)
    }

    v.Invoice.StartDate = 0
    v.Invoice.EndDate = len(v.Invoice.Entries) - 1
    v.Invoice.Version = 5

    return v, nil
}

func (v *Invoice) Save(path string) error {
    b, err := json.MarshalIndent(v, "", "    ")
    if err != nil {
        return err
    }

    err = ioutil.WriteFile(path, b, 0644)
    if err != nil {
        return err
    }

    return nil
}

func (v *Invoice) SetHours(key string, hours int) {
    for i := 0; i < len(v.Invoice.Entries); i++ {
        if v.Invoice.Entries[i].Key == key {
            v.Invoice.Entries[i].Hours = hours
            v.SetTotal()
            return
        }
    }
    return
}

func (v *Invoice) SetSelected(key string) {
    today := time.Now().Format("2006-01-02")
    if key == "" {
        key = today
    }

    for i, _ := range v.Invoice.Entries {
        v.Invoice.Entries[i].Selected = false
        v.Invoice.Entries[i].Today = false
    }

    found := false
    for i, entry := range v.Invoice.Entries {
        if entry.Key == key {
            v.Invoice.Selected = i
            v.Invoice.SelectedInRange = true
            v.Invoice.Entries[i].Selected = true
            found = true
        }
        if entry.Key == today {
            v.Invoice.Entries[i].Today = true
        }
    }

    if !found {
        v.Invoice.Selected = 0
        v.Invoice.Entries[0].Selected = true
    }

    return
}

func (v *Invoice) SetTotal() {
    total := 0
    for _, entry := range v.Invoice.Entries {
        total = total + entry.Hours
    }
    v.Invoice.Total = total
}

