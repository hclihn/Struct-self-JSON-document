package main

import (
	"fmt"
	"reflect"
	"strings"
  "time"
  "strconv"
  "unicode"
)

type ResetAction int

// ResetAction enum
const (
	ResetActionNone ResetAction = iota
	ResetActionReboot
	ResetActionHostPoserCycle
	ResetActionMBPowerCycle
)

func (e *ResetAction) GetStrings() []string {
	return []string{"No_Reset", "Reboot_Host", "Reset_Host_Power", "Reset_Motherboard_Power"}
}

type TimestampUTC struct {
  time.Time
}

type F struct {
	FormatVersion string `cabdoc:"string representing format:version"`
  format string
  version int
}

type Info struct {
	Description string       `cabdoc:"description"`
	LastChanged TimestampUTC `cabdoc:"last modified timestamp in the format of YYYYMMDDThhmmssZ" cabdef:"\"20200123T123456Z\""`
	ChangedBy   string       `cabdoc:"last changed by (person)"`
  Cmd CmdOptionInfo `cabdoc:"strange command"`
}

type CmdOptionInfo struct {
	Name string `cabdoc:"command option name (can be a template string)"`
	After []string `cabdoc:"option(s) preceed this command option"`
}

type GroupInfo struct {
	Name       string   `cabdoc:"name of the update group"`
	Order      []string `cabdoc:"list of template strings evaluated to component type in update order"`
	ResetAfter string   `cabdoc:"template string evaluates to the reset action (immediately after the group update) string" cabenum:"resetAfter,0"`
  resetAfter ResetAction 
}

type CapMap map[string]int
type JSONBool int

type T struct {
	F   `cabdef:"\"image:1\""`
  Info
	Type     string `cabdoc:"component type string"`
	Model     string `cabdoc:"component model string"`
	DeviceID     string `cabdoc:"template string to get device ID"`
	ForceOption CmdOptionInfo `cabdoc:"information for the forced command option"`
	Capacity int64  `cabdoc:"capacity in Watts (integer)"`
	ResetByExitCodes []JSONBool `cabdoc:"a list of exit codes to trigger ResetAfter"`
	Capability CapMap `cabdoc:"capability map of key:value"`
  Groups         [][]GroupInfo   `cabdoc:"group update info"`
  ResetAfter   ResetAction `cabdoc:"Reset after this" cabenum:"ResetAfter,1"`
}

type TagInfo struct {
  DocStr string
  Default string
  Enum []string
  EnumIdx int
  Required bool
  Readonly bool
  EmptyOk bool
}

func (g *TagInfo) GetEnumField(t string, st reflect.Type) error {
  if t == "" {
    return nil
  }
  tFields := strings.Split(t, ",")
  if len(tFields) != 2 {
    return fmt.Errorf("the cabenum tag should contain 2 comma-separated fields, got %q", t)
  }
  ei, err := strconv.Atoi(tFields[1])
  if err != nil {
    return fmt.Errorf("failed to parse enum index %q from cabenum tag %q: %w", tFields[1], t, err)
  }
  en := strings.TrimSpace(tFields[0])
  sf, ok := st.FieldByName(en)
  if !ok {
    return fmt.Errorf("failed to get enum field %q from cabenum tag %q", en, t)
  }
  sft := GetRefObj(sf.Type)
  m, ok := sft.MethodByName("GetStrings")
  if !ok {
    sft = reflect.PtrTo(sft)
    fmt.Println("Looking up method in the pointer type side...")
    if m, ok = sft.MethodByName("GetStrings"); !ok {
      return fmt.Errorf("failed to get enum %q from cabenum tag %q: no method GetStrings()", sf.Name, t)
    }
  }

  in := make([]reflect.Value, m.Type.NumIn())
  in[0] = reflect.New(sft).Elem()
  s := m.Func.Call(in)[0].Interface().([]string)
  if ei < 0 || ei >= len(s) {
    return fmt.Errorf("failed to get enum %q from cabenum tag %q: index %d out of range [0, %d]", sf.Name, t, ei, len(s)-1)
  }
  g.Enum = s
  g.EnumIdx = ei
  return nil
}

func (g *TagInfo) AddToDoc(s string) {
  g.DocStr = strings.TrimRightFunc(g.DocStr, unicode.IsSpace)
  if g.DocStr != "" {
    if !strings.HasSuffix(g.DocStr, ".") {
      g.DocStr += "."
    }
    g.DocStr += " "
  }
  g.DocStr += s
}

func (g *TagInfo) Parse(tg reflect.StructTag, st reflect.Type) error {
  if s, ok := tg.Lookup("cabdoc"); ok {
    g.DocStr = s
  }
  if s, ok := tg.Lookup("cabdef"); ok {
    g.Default = s
  }
  if f, ok := tg.Lookup("cabflag"); ok {
    for i, flag := range strings.Split(f, ",") {
      flag = strings.TrimSpace(flag)
      switch flag {
      case "req":
        g.Required = true
        g.AddToDoc("Required.")
      case "ro":
        g.Readonly = true
        g.AddToDoc("Read only.")
      case "emptyok":
        g.EmptyOk = true
      default:
        return fmt.Errorf("failed to parse cabflag tag %q entry #%d: undefined flag %q", f, i, flag)
      }
    }
  }
  if err := g.GetEnumField(tg.Get("cabenum"), st); err != nil {
    return fmt.Errorf("failed to parse cabenum tag: %w", err)
  }
  if g.Default == "" && len(g.Enum) > 0 {
    g.Default = fmt.Sprintf(`"%s"`, g.Enum[g.EnumIdx])
  }
  if len(g.Enum) > 0 {
    vals := strings.Join(g.Enum, ", ")
    g.AddToDoc(fmt.Sprintf("Defined values are: %s.", vals)) 
  }
  return nil
}

func IfElese(cond bool, a, b interface{}) interface{} {
  if cond {
    return a
  }
  return b
}

func GetRefObj(t reflect.Type) reflect.Type {
getValue:
	for { // move through interface and pointers to get to the actual object
		switch t.Kind() {
		case reflect.Interface, reflect.Ptr:
			  t = t.Elem()
		default:
			break getValue
		}
	}
  return t
}

func serializeTypeJSON(buf *strings.Builder, rt reflect.Type, level int, anonymous bool, ti *TagInfo) error {
  rt = GetRefObj(rt)
  fmt.Printf("%s: Name=%q Kind=%q, Pkg=%q\n", rt, rt.Name(), rt.Kind(), rt.PkgPath())
  switch k := rt.Kind(); k {
  case reflect.Bool:
    buf.WriteString("false")
  case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, 
    reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
    buf.WriteString("0")
  case reflect.Float32, reflect.Float64:
    buf.WriteString("0.0")
  case reflect.String:
    buf.WriteString(`""`)
  case reflect.Struct:
      serializeStructJSON(buf, reflect.Zero(rt).Interface(), level + IfElese(anonymous, 0, 1).(int), anonymous, ti)
  case reflect.Map:
    buf.WriteByte('{')
    if err := serializeTypeJSON(buf, rt.Key(), level, anonymous, ti); err != nil {
      return fmt.Errorf("failed to serialize map key type (%s): %w", rt.Key().Kind(), err)
    }
    buf.WriteByte(':')
    if err := serializeTypeJSON(buf, rt.Elem(), level, anonymous, ti); err != nil {
      return fmt.Errorf("failed to serialize map value type (%s): %w", rt.Elem().Kind(), err)
    }
    buf.WriteByte('}')
  case reflect.Array, reflect.Slice:
    buf.WriteByte('[')
    if err := serializeTypeJSON(buf, rt.Elem(), level, anonymous, ti); err != nil {
      return fmt.Errorf("failed to serialize slice element type (%s): %w", rt.Elem().Kind(), err)
    }
    buf.WriteByte(']')
  default: // unsupported field
    return fmt.Errorf("the type (%s) is not supported", k)
  }
  return nil
}

func serializeStructJSON(buf *strings.Builder, tObj interface{}, level int, anonymous bool, ti *TagInfo) error {
	prefix := strings.Repeat(" ", level * 2)
  t := GetRefObj(reflect.TypeOf(tObj))
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("the object (%T) is not a struct type", tObj)
	}
	if !anonymous {
		buf.WriteString("{\n")
	}
	for i, nf := 0, t.NumField(); i < nf; i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			fmt.Printf("non-exporting field %s skipped\n", f.Name)
			continue
		}
    var tgi TagInfo 
    if ti != nil {
      tgi = *ti
    }
    if err := tgi.Parse(f.Tag, t); err != nil {
        return fmt.Errorf("failed to parse tag for field %q: %w", f.Name, err)
    }
    if !f.Anonymous {
		  fmt.Fprintf(buf, "%s  %q: ", prefix, f.Name)
    } 
    if !f.Anonymous && tgi.Default != "" {
      fmt.Fprintf(buf, "%s", tgi.Default)
    } else { 
      pt := IfElese(f.Anonymous, &tgi, (*TagInfo)(nil)).(*TagInfo)
      if err := serializeTypeJSON(buf, f.Type, level, f.Anonymous, pt); err != nil {
        return fmt.Errorf("the object (%T) field #%d type is not supported: %w", tObj, i, err)
      }
    }
		if !f.Anonymous && i < nf-IfElese(anonymous, 0, 1).(int) {
			buf.WriteByte(',')
		}
    if tgi.DocStr != "" {
		  fmt.Fprintf(buf, " /* %s */", tgi.DocStr)
    }
    if !f.Anonymous {
		  buf.WriteByte('\n')
    }
	}
	if !anonymous {
		buf.WriteString(prefix+"}")
	}
	return nil
}

func SerializeStructJSON(tObj interface{}) string {
  var buf strings.Builder
  serializeStructJSON(&buf, tObj, 0, false, nil)
  return buf.String()
}

func main() {	
  var tt T
	fmt.Println(SerializeStructJSON(&tt))
}