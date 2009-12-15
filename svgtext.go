package svgtext

/* SVG driver (text) for graf.go.

Copyright (c) 2009 Helmar Wodtke

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

import (
  "pdfread";
  "graf";
  "util";
  "strm";
  "io/ioutil";
  "cmapt";
  "cmapi";
  "fancy";
  "ps";
)

const WIDTH_DENSITY = 10000

type SvgTextT struct {
  Pdf      *pdfread.PdfReaderT;
  Drw      *graf.PdfDrawerT;
  Write    util.OutT;
  Page     int;
  matrix   []string;
  fonts    pdfread.DictionaryT;
  fontw    map[string][]int64;
  fontW    map[string]*cmapt.CMapT;
  x0, x, y string;
  cmaps    map[string]*cmapi.CharMapperT;
}

func New(pdf *pdfread.PdfReaderT, drw *graf.PdfDrawerT) *SvgTextT {
  r := new(SvgTextT);
  drw.Text = r;
  r.Drw = drw;
  r.Pdf = pdf;
  r.TSetMatrix(nil);
  r.cmaps = make(map[string]*cmapi.CharMapperT);
  return r;
}

// ------------------------------------------------ Font Substitution

const DEFAULT_FSTYLE = "font-family:Arial;"

func csvtok(d []byte) []byte {
  p := 0;
  for ; p < len(d); p++ {
    if d[p] < 32 {
      break
    }
  }
  return d[0:p];
}

func endcsvl(d []byte) int {
  p := 0;
  for ; p < len(d); p++ {
    if d[p] == 10 {
      break
    }
  }
  return p + 1;
}

var fileNames, // will be needed for SVG font inclusion
  styles map[string]string

func fontnamemap(fn string) int {
  if fileNames == nil {
    fileNames = make(map[string]string)
  }
  if styles == nil {
    styles = make(map[string]string)
  }

  data, _ := ioutil.ReadFile(fn);
  no := 0;
  for p := 0; p < len(data); {
    n := string(csvtok(data[p:]));
    p += len(n) + 1;
    f := string(csvtok(data[p:]));
    p += len(f) + 1;
    s := string(csvtok(data[p:]));
    p += len(s);
    p += endcsvl(data[p:]);
    fileNames[n] = f;
    styles[n] = s;
    no++;
  }
  return no;
}

var numFonts = fontnamemap("fontnamemap.txt") // initialize fileNames and styles

func FStyle(f string) string {
  if f[0] == '/' {
    f = f[1:]
  }
  if r, ok := styles[f]; ok {
    return r
  }
  q := 0;
  for ; q < len(f); q++ {
    if f[q] == '+' {
      break
    }
  }
  if q < len(f) {
    f = f[q+1 : len(f)]
  }
  if r, ok := styles[f]; ok {
    return r
  }
  return DEFAULT_FSTYLE;
}

// ------------------------------------------------

func (t *SvgTextT) Style(font string) (r string) {
  r = DEFAULT_FSTYLE;
  if t.fonts == nil {
    t.fonts = t.Pdf.PageFonts(t.Pdf.Pages()[t.Page]);
    if t.fonts == nil {
      return
    }
  }
  if dr, ok := t.fonts[font]; ok {
    d := t.Pdf.Dic(dr);
    if fd, ok := d["/FontDescriptor"]; ok { // FIXME: Too simple...
      return FStyle(string(t.Pdf.Dic(fd)["/FontName"]))
    }
  }
  return;
}

func (t *SvgTextT) widths(font string) (rW *cmapt.CMapT) {
  if t.fontW == nil {
    t.fontW = make(map[string]*cmapt.CMapT)
  } else if rW, ok := t.fontW[font]; ok {
    return rW
  }
  // initialize like for Courier.
  rW = cmapt.New();
  rW.AddDef(0, 256, 600*WIDTH_DENSITY/1000);
  if t.fonts == nil {
    t.fonts = t.Pdf.PageFonts(t.Pdf.Pages()[t.Page]);
    if t.fonts == nil {
      return
    }
  }
  if dr, ok := t.fonts[font]; ok {
    d := t.Pdf.Dic(dr);
    fc, ok := d["/FirstChar"];
    if !ok {
      return
    }
    lc, ok := d["/LastChar"];
    if !ok {
      return
    }
    wd, ok := d["/Widths"];
    if !ok {
      return
    }
    p := strm.Int(string(fc), 1);
    q := strm.Int(string(lc), 1);
    a := t.Pdf.Arr(wd);
    for k := p; k < q; k++ {
      rW.Add(k, strm.Int(string(a[k-p]), WIDTH_DENSITY/1000))
    }
  }
  return;
}

var cm_identity = cmapi.Read(nil)

func (t *SvgTextT) cmap(font string) (r *cmapi.CharMapperT) {
  var ok bool;
  if r, ok = t.cmaps[font]; ok {
    return
  }
  r = cm_identity; // setup default
  if t.fonts == nil {
    t.fonts = t.Pdf.PageFonts(t.Pdf.Pages()[t.Page]);
    if t.fonts == nil {
      return
    }
  }
  if dr, ok := t.fonts[font]; ok {
    d := t.Pdf.Dic(dr);
    if tu, ok := d["/ToUnicode"]; ok {
      _, cm := t.Pdf.DecodedStream(tu);
      r = cmapi.Read(fancy.SliceReader(cm));
      t.cmaps[font] = r;
    }
  }
  return;
}

func (t *SvgTextT) Utf8TsAdvance(s []byte) ([]byte, int64) {
  W := t.widths(t.Drw.TConfD.Font);
  width := int64(0);
  for k := range s {
    width += int64(W.Code(int(s[k])))
  }
  return cmapi.Decode(s, t.cmap(t.Drw.TConfD.Font)), width;
}

func (t *SvgTextT) Utf8Advance(s []byte) ([]byte, string) {
  r, a := t.Utf8TsAdvance(s);
  return r, strm.Mul(t.Drw.TConfD.FontSize, strm.String(a, WIDTH_DENSITY));
}

func (t *SvgTextT) TMoveTo(s [][]byte) {
  t.x0 = strm.Add(t.x0, string(s[0]));
  t.x = t.x0;
  t.y = strm.Sub(t.y, string(s[1]));
}

func (t *SvgTextT) TNextLine() {
  t.x = t.x0;
  t.y = strm.Add(t.y, t.Drw.TConfD.Leading);
}

func (t *SvgTextT) TSetMatrix(s [][]byte) {
  if s == nil {
    t.matrix = []string{"1", "0", "0", "1", "0", "0"}
  } else {
    t.matrix = util.StringArray(s)
  }
  t.x0 = "0";
  t.x = t.x0;
  t.y = "0";
}

func space_split(a []byte) [][]byte {
  c := 1;
  for p := 0; p < len(a); {
    for ; p < len(a) && a[p] == 32; p++ {
    }
    for ; p < len(a) && a[p] != 32; p++ {
    }
    if p < len(a)-2 && a[p+1] == 32 {
      c++
    }
  }
  r := make([][]byte, c);
  c = 0;
  q := 0;
  for p := 0; p < len(a); {
    for ; p < len(a) && a[p] == 32; p++ {
    }
    for ; p < len(a) && a[p] != 32; p++ {
    }
    if p < len(a)-2 && a[p+1] == 32 {
      r[c] = a[q:p];
      q = p;
      c++;
    }
  }
  r[c] = a[q:];
  return r;
}

func (t *SvgTextT) TShow(a []byte) {
  tx := t.Pdf.ForcedArray(a); // FIXME: Should be "ForcedSimpleArray()"
  for k := range tx {
    if tx[k][0] == '(' || tx[k][0] == '<' {
      part := space_split(ps.String(tx[k]));
      for y := range part {
        tmp, adv := t.Utf8Advance(part[y]);
        res := strm.Add(t.x, adv);
        p := 0;
        for len(tmp) > p && tmp[p] == 32 {
          p++
        }
        if p > 0 {
          _, ta := t.Utf8Advance(tmp[0:p]);
          t.x = strm.Add(t.x, ta);
          tmp = tmp[p:];
        }
        t.Drw.Write.Out(
          "<g transform=\"matrix(%s,%s,%s,%s,%s,%s)\">\n"+
            "<text x=\"%s\" y=\"%s\""+
            " font-size=\"%s\""+
            " style=\"stroke:none;%v\""+
            " fill=\"%s\">%s</text>\n"+
            "</g>\n",
          t.matrix[0], t.matrix[1],
          strm.Neg(t.matrix[2]), strm.Neg(t.matrix[3]),
          t.matrix[4], t.matrix[5],
          t.x, t.y,
          t.Drw.TConfD.FontSize,
          t.Style(t.Drw.TConfD.Font),
          t.Drw.ConfigD.FillColor,
          util.ToXML(tmp));
        t.x = res;
      }
    } else {
      t.x = strm.Sub(t.x, strm.Mul(string(tx[k]), "0.01"))
    }
  }
}
