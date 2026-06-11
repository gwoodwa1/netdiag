package interactive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/gwoodwa1/netdiag/internal/model"
)

type documentData struct {
	Title  string      `json:"title"`
	Nodes  []nodeData  `json:"nodes"`
	Groups []groupData `json:"groups"`
	Links  []linkData  `json:"links"`
}

type nodeData struct {
	ID        string                 `json:"id"`
	DOMID     string                 `json:"domId"`
	Label     string                 `json:"label"`
	Role      string                 `json:"role"`
	Icon      string                 `json:"icon,omitempty"`
	IconLabel string                 `json:"iconLabel,omitempty"`
	Color     string                 `json:"color,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type groupData struct {
	ID       string   `json:"id"`
	DOMID    string   `json:"domId"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"`
	ParentID string   `json:"parentId,omitempty"`
	NodeIDs  []string `json:"nodeIds"`
}

type linkData struct {
	ID           string   `json:"id"`
	From         string   `json:"from"`
	FromPort     string   `json:"fromPort"`
	FromAddress  string   `json:"fromAddress,omitempty"`
	To           string   `json:"to"`
	ToPort       string   `json:"toPort"`
	ToAddress    string   `json:"toAddress,omitempty"`
	Label        string   `json:"label,omitempty"`
	Style        string   `json:"style,omitempty"`
	Bundle       string   `json:"bundle,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	MultiChassis bool     `json:"multiChassis,omitempty"`
}

// Render wraps a native netdiag SVG and its model in a dependency-free,
// single-file interactive preview.
func Render(diagram *model.Diagram, svg []byte) ([]byte, error) {
	data := buildData(diagram)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	fmt.Fprintf(&out, "<title>%s</title>", htmlEscape(data.Title))
	out.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	out.WriteString(`<style>
*{box-sizing:border-box}body{margin:0;font-family:Inter,Segoe UI,Arial,sans-serif;color:#0f172a;background:#e2e8f0;overflow:hidden}
#app{height:100vh;display:grid;grid-template-columns:minmax(0,1fr) 340px}
#stage{position:relative;overflow:hidden;background:#cbd5e1;cursor:grab}#stage.dragging{cursor:grabbing}
#stage svg{position:absolute;inset:0;width:100%;height:100%;user-select:none}
#toolbar{position:absolute;z-index:3;top:14px;left:14px;display:flex;gap:7px;padding:7px;border:1px solid #cbd5e1;border-radius:10px;background:#ffffffeb;box-shadow:0 5px 18px #0f172a24}
button{border:1px solid #cbd5e1;border-radius:7px;background:#fff;color:#0f172a;padding:7px 10px;font-weight:700;cursor:pointer}button:hover{background:#eff6ff}
#sidebar{overflow:auto;background:#fff;border-left:1px solid #cbd5e1;padding:20px}h1{font-size:18px;margin:0 0 5px}h2{font-size:13px;text-transform:uppercase;letter-spacing:.1em;color:#64748b;margin:24px 0 10px}
.hint{font-size:12px;color:#64748b;line-height:1.45}.group{display:flex;width:100%;justify-content:space-between;margin:6px 0;text-align:left}
.group.collapsed{background:#e2e8f0;color:#64748b}dl{margin:0}dt{font-size:10px;font-weight:800;color:#64748b;text-transform:uppercase;letter-spacing:.08em;margin-top:12px}
dd{margin:3px 0 0;white-space:pre-wrap;word-break:break-word;font:12px/1.45 ui-monospace,SFMono-Regular,Consolas,monospace}
[data-netdiag-kind="node"],[data-netdiag-kind="link"],[data-netdiag-kind="group"]{cursor:pointer}
[data-netdiag-kind="node"]:hover,[data-netdiag-kind="group"]:hover{outline:3px solid #38bdf8;outline-offset:3px}
.selected{outline:4px solid #f59e0b!important;outline-offset:4px}.is-hidden{display:none}
</style></head><body><div id="app"><main id="stage"><div id="toolbar"><button id="fit">Fit</button><button id="zin">+</button><button id="zout">−</button></div>`)
	out.Write(svg)
	out.WriteString(`</main><aside id="sidebar"><h1 id="title"></h1><p class="hint">Wheel to zoom, drag to pan, and click a node, link, or group to inspect it.</p><h2>Groups</h2><div id="groups"></div><h2>Inspector</h2><div id="inspector" class="hint">Select an element in the diagram.</div></aside></div>`)
	fmt.Fprintf(&out, `<script id="netdiag-data" type="application/json">%s</script>`, jsonData)
	out.WriteString(`<script>
const data=JSON.parse(document.getElementById('netdiag-data').textContent),stage=document.getElementById('stage'),svg=stage.querySelector('svg');
const title=document.getElementById('title'),groupsEl=document.getElementById('groups'),inspector=document.getElementById('inspector');
title.textContent=data.title||'netdiag interactive preview';
const original=svg.viewBox.baseVal,view={x:original.x,y:original.y,w:original.width,h:original.height},collapsed=new Set();
function apply(){svg.setAttribute('viewBox',[view.x,view.y,view.w,view.h].join(' '))}
function zoom(f,cx=stage.clientWidth/2,cy=stage.clientHeight/2){const r=stage.getBoundingClientRect(),px=view.x+(cx-r.left)/r.width*view.w,py=view.y+(cy-r.top)/r.height*view.h,nw=view.w*f,nh=view.h*f;view.x=px-(px-view.x)*f;view.y=py-(py-view.y)*f;view.w=nw;view.h=nh;apply()}
function fit(){view.x=original.x;view.y=original.y;view.w=original.width;view.h=original.height;apply()}
stage.addEventListener('wheel',e=>{e.preventDefault();zoom(e.deltaY>0?1.12:.89,e.clientX,e.clientY)},{passive:false});
let drag=null;stage.addEventListener('pointerdown',e=>{if(e.target.closest('[data-netdiag-kind]'))return;drag={x:e.clientX,y:e.clientY,vx:view.x,vy:view.y};stage.classList.add('dragging');stage.setPointerCapture(e.pointerId)});
stage.addEventListener('pointermove',e=>{if(!drag)return;view.x=drag.vx-(e.clientX-drag.x)/stage.clientWidth*view.w;view.y=drag.vy-(e.clientY-drag.y)/stage.clientHeight*view.h;apply()});
stage.addEventListener('pointerup',()=>{drag=null;stage.classList.remove('dragging')});
document.getElementById('fit').onclick=fit;document.getElementById('zin').onclick=()=>zoom(.8);document.getElementById('zout').onclick=()=>zoom(1.25);
const byId=(items,id)=>items.find(x=>x.id===id),details=(kind,item)=>{document.querySelectorAll('.selected').forEach(x=>x.classList.remove('selected'));const el=document.getElementById(item.domId||item.id);if(el)el.classList.add('selected');const rows=Object.entries(item).filter(([k,v])=>k!=='domId'&&v!==''&&v!=null&&(!Array.isArray(v)||v.length));inspector.innerHTML='<strong>'+kind.toUpperCase()+'</strong><dl>'+rows.map(([k,v])=>'<dt>'+k+'</dt><dd>'+escapeHTML(typeof v==='object'?JSON.stringify(v,null,2):String(v))+'</dd>').join('')+'</dl>'};
const escapeHTML=s=>s.replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function refresh(){const hidden=new Set();for(const id of collapsed){const g=byId(data.groups,id);if(g)g.nodeIds.forEach(n=>hidden.add(n))}data.nodes.forEach(n=>document.getElementById(n.domId)?.classList.toggle('is-hidden',hidden.has(n.id)));data.links.forEach(l=>document.getElementById(l.id)?.classList.toggle('is-hidden',hidden.has(l.from)||hidden.has(l.to)));data.groups.forEach(g=>document.getElementById(g.domId)?.classList.toggle('is-hidden',collapsed.has(g.id)));document.querySelectorAll('.group').forEach(b=>b.classList.toggle('collapsed',collapsed.has(b.dataset.id)))}
data.groups.filter(g=>!g.parentId).forEach(g=>{const b=document.createElement('button');b.className='group';b.dataset.id=g.id;b.innerHTML='<span>'+escapeHTML(g.label||g.id)+'</span><span>'+g.nodeIds.length+'</span>';b.onclick=()=>{collapsed.has(g.id)?collapsed.delete(g.id):collapsed.add(g.id);refresh()};groupsEl.appendChild(b)});
stage.addEventListener('click',e=>{const el=e.target.closest('[data-netdiag-kind]');if(!el)return;e.stopPropagation();const kind=el.dataset.netdiagKind;if(kind==='node')details(kind,data.nodes.find(x=>x.domId===el.id));else if(kind==='link')details(kind,byId(data.links,el.id));else details(kind,data.groups.find(x=>x.domId===el.id))});
apply();
</script></body></html>`)
	return out.Bytes(), nil
}

func buildData(diagram *model.Diagram) documentData {
	data := documentData{Title: diagram.Theme.Title}
	for _, node := range diagram.Nodes {
		data.Nodes = append(data.Nodes, nodeData{ID: node.ID, DOMID: svgID(node.ID), Label: node.Label, Role: node.Role, Icon: node.Icon, IconLabel: node.IconLabel, Color: node.Color, Metadata: node.Metadata})
	}
	children := make(map[string][]model.Group)
	for _, group := range diagram.Groups {
		children[group.ParentID] = append(children[group.ParentID], group)
	}
	var descendantNodes func(model.Group) []string
	descendantNodes = func(group model.Group) []string {
		nodeIDs := append([]string{}, group.NodeIDs...)
		for _, child := range children[group.ID] {
			nodeIDs = append(nodeIDs, descendantNodes(child)...)
		}
		return nodeIDs
	}
	for _, group := range diagram.Groups {
		nodeIDs := descendantNodes(group)
		sort.Strings(nodeIDs)
		data.Groups = append(data.Groups, groupData{ID: group.ID, DOMID: "group-" + svgID(group.ID), Label: group.Label, Kind: group.Kind, ParentID: group.ParentID, NodeIDs: nodeIDs})
	}
	for index, link := range diagram.Links {
		data.Links = append(data.Links, linkData{
			ID: fmt.Sprintf("link-%d", index+1), From: link.From.Node, FromPort: link.From.Port, FromAddress: link.From.Address,
			To: link.To.Node, ToPort: link.To.Port, ToAddress: link.To.Address, Label: link.MiddleLabel(), Style: link.Style,
			Bundle: link.Bundle, Tags: link.Tags(), MultiChassis: link.MultiChassis,
		})
	}
	return data
}

func svgID(value string) string {
	var out bytes.Buffer
	for _, r := range value {
		switch r {
		case ' ', '/', ':':
			out.WriteByte('-')
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func htmlEscape(value string) string {
	var out bytes.Buffer
	for _, r := range value {
		switch r {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '>':
			out.WriteString("&gt;")
		case '"':
			out.WriteString("&quot;")
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
