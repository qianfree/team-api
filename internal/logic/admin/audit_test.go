package admin

import (
	"encoding/json"
	"testing"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestToMapListProducesJSONObjects(t *testing.T) {
	t.Parallel()

	result := gdb.Result{
		gdb.Record{
			"id":            gvar.New(9),
			"error_message": gvar.New("request failed"),
		},
	}

	list := toMapList(result)
	if len(list) != 1 {
		t.Fatalf("toMapList() length = %d, want 1", len(list))
	}
	if list[0] == nil {
		t.Fatal("toMapList() returned a nil map")
	}

	data, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal list: %v", err)
	}
	if got, want := string(data), `[{"error_message":"request failed","id":9}]`; got != want {
		t.Fatalf("marshal list = %s, want %s", got, want)
	}
}
