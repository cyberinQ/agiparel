package web
import("strings";"testing")
func TestStoresWorkspaceAndLinks(t *testing.T){
 for _,name:=range []string{"stores.html","stores.css","stores.js"}{if _,err:=staticFS.ReadFile("static/"+name);err!=nil{t.Fatal(err)}}
 b,err:=GetStoresHTML();if err!=nil{t.Fatal(err)}
 for _,needle:=range []string{"/static/stores.js","/static/workspace_nav.js","/documents","/material-planning","does not create available stock","roll_id","inspection_result"}{if !strings.Contains(strings.ToLower(string(b)),strings.ToLower(needle)){t.Errorf("missing %q",needle)}}
 nav,_:=staticFS.ReadFile("static/workspace_nav.js");if !strings.Contains(string(nav),"'/stores'"){t.Error("stores missing in shared navigation")}
 portal,_:=GetPortalHTML();if !strings.Contains(string(portal),`href="/stores"`){t.Error("stores missing from homepage")}
}
