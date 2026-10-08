package client

import "testing"

func TestBuiltinVersions(t *testing.T) {
	vs := Versions()
	if len(vs) < 100 {
		t.Fatalf("only %d versions", len(vs))
	}
	v, ok := FindBySignatures(0x42A3, 0x57BBD603)
	if !ok || v.Value != 1098 || v.Name != "10.98" {
		t.Fatalf("10.98 lookup = %+v %v", v, ok)
	}
	if got := FindByValue(854); len(got) != 3 {
		t.Fatalf("8.54 variants = %d", len(got))
	}
	if _, ok := FindBySignatures(1, 2); ok {
		t.Fatal("unexpected match")
	}
	vs[0].Name = "mutated"
	if Versions()[0].Name == "mutated" {
		t.Fatal("Versions must return a copy")
	}
}

func TestParseVersionsErrors(t *testing.T) {
	if _, err := ParseVersions([]byte(`<versions><version value="x" string="a" dat="1" spr="1"/></versions>`)); err == nil {
		t.Fatal("expected error for bad value")
	}
	if _, err := ParseVersions([]byte(`<versions><version value="1" string="a" dat="zz" spr="1"/></versions>`)); err == nil {
		t.Fatal("expected error for bad signature")
	}
}

func TestFeatureDefaults(t *testing.T) {
	cases := []struct {
		v                 uint16
		ext, anim, groups bool
	}{
		{740, false, false, false},
		{960, true, false, false},
		{1050, true, true, false},
		{1057, true, true, true},
		{1098, true, true, true},
	}
	for _, c := range cases {
		f := DefaultFeatures(c.v)
		if f.Extended != c.ext || f.ImprovedAnimations != c.anim || f.FrameGroups != c.groups || f.SpriteSize != 32 {
			t.Errorf("%d: %+v", c.v, f)
		}
	}
	f := Features{Transparency: true, Extended: true}
	f.ApplyVersionDefaults(710)
	if !f.Extended || !f.Transparency {
		t.Fatal("ApplyVersionDefaults must not turn features off")
	}
}

func TestMetadataFormat(t *testing.T) {
	cases := map[uint16]int{710: 1, 730: 1, 740: 2, 750: 2, 755: 3, 772: 3, 780: 4, 854: 4, 860: 5, 986: 5, 1010: 6, 1310: 6}
	for v, want := range cases {
		if got := MetadataFormat(v); got != want {
			t.Errorf("MetadataFormat(%d) = %d, want %d", v, got, want)
		}
	}
}

func TestOTFIRoundTrip(t *testing.T) {
	src := []byte("// comment\nDatSpr\n  extended: true\n  transparency: true\n  frame-durations: false\n  frame-groups: true\n  metadata-file: \"Tibia.dat\"\n  sprites-file: Tibia.spr\n  sprite-size: 64\n")
	o, err := ParseOTFI(src)
	if err != nil {
		t.Fatal(err)
	}
	want := OTFI{Features: Features{Extended: true, Transparency: true, FrameGroups: true, SpriteSize: 64}, MetadataFile: "Tibia.dat", SpritesFile: "Tibia.spr"}
	if o != want {
		t.Fatalf("parsed %+v", o)
	}
	back, err := ParseOTFI(o.Marshal())
	if err != nil || back != o {
		t.Fatalf("round trip %+v %v", back, err)
	}
}

func TestOTFIErrors(t *testing.T) {
	if _, err := ParseOTFI([]byte("Other\n  a: b\n")); err == nil {
		t.Fatal("expected missing node error")
	}
	if _, err := ParseOTFI([]byte("DatSpr\n  sprite-size: abc\n")); err == nil {
		t.Fatal("expected sprite-size error")
	}
	o, err := ParseOTFI([]byte("DatSpr\n  extended: true\n"))
	if err != nil || o.Features.SpriteSize != 32 {
		t.Fatalf("default sprite size: %+v %v", o, err)
	}
}
