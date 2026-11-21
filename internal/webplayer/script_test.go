package webplayer

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestPlayerScriptAsyncPauseAndReporting(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is only needed for the optional browser-script regression checks")
	}
	html := playerHTML("https://example.invalid/video.mp4", "/state?token=fixture")
	script := strings.Split(strings.Split(html, "<script>")[1], "</script>")[0]
	input, _ := json.Marshal(map[string]string{"script": script})
	cmd := exec.Command(node, "-e", scriptHarness)
	cmd.Stdin = strings.NewReader(string(input))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("browser script regression: %v\n%s", err, out)
	}
}

func TestPlayerControlsScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is only needed for browser-script regression checks")
	}
	html := playerHTML("https://example.invalid/video.mp4", "/state?token=fixture")
	script := strings.Split(strings.Split(html, "<script>")[1], "</script>")[0]
	input, _ := json.Marshal(map[string]string{"script": script})
	cmd := exec.Command(node, "-e", controlsHarness)
	cmd.Stdin = strings.NewReader(string(input))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("controls regression: %v\n%s", err, out)
	}
}

func TestPlayerMediaLifecycleScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is only needed for browser-script regression checks")
	}
	html := playerHTML("https://example.invalid/video.mp4", "/state?token=fixture")
	script := strings.Split(strings.Split(html, "<script>")[1], "</script>")[0]
	input, _ := json.Marshal(map[string]string{"script": script})
	cmd := exec.Command(node, "-e", lifecycleHarness)
	cmd.Stdin = strings.NewReader(string(input))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("media lifecycle regression: %v\n%s", err, out)
	}
}

const lifecycleHarness = `
const vm=require('node:vm'),assert=require('node:assert/strict');
const {script}=JSON.parse(require('node:fs').readFileSync(0,'utf8'));
class Element {
 constructor(tag='DIV'){this.tagName=tag;this.listeners={};this.children=[];this.attrs={};this.hidden=false;this.value='0';this.textContent='';this.style={setProperty:(k,v)=>this.attrs[k]=v};this.dataset={};this.classes=new Set();this.classList={add:x=>this.classes.add(x),remove:x=>this.classes.delete(x),toggle:(x,on)=>on?this.classes.add(x):this.classes.delete(x)};}
 addEventListener(name,fn){(this.listeners[name]??=[]).push(fn);}
 emit(name,extra={}){const event={target:this,preventDefault(){this.prevented=true},...extra};for(const fn of this.listeners[name]||[])fn(event);return event;}
 appendChild(el){this.children.push(el);return el;}
 get firstChild(){return this.children[0]||null;}
 removeChild(el){const i=this.children.indexOf(el);if(i>=0)this.children.splice(i,1);}
 replaceChildren(){this.children=[];}
 remove(){}
 after(el){this.afterNode=el;}
 setAttribute(k,v){this.attrs[k]=v;}
 getBoundingClientRect(){return this.rect||{left:0,top:0,width:0,height:0};}
 get offsetWidth(){return (this.rect&&this.rect.width)||0;}
 scrollIntoView(opts){this.scrolledTo=opts;}
 contains(el){return el===this||this.children.some(x=>x.contains(el));}
}
const els={};const get=id=>els[id]??=(new Element());
const created=[];
const pending=[];
// 逼真的媒体元素：load() 按规范把 playbackRate 复位成 defaultPlaybackRate，
// play() 返回未完成的 Promise，用来验证换集时旧 play() 会被作废。
const asMedia=el=>Object.assign(el,{currentTime:0,duration:240,readyState:4,error:null,paused:true,ended:false,muted:false,volume:1,playbackRate:1,defaultPlaybackRate:1,plays:0,
 load(){this.readyState=0;this.playbackRate=this.defaultPlaybackRate;},
 play(){this.plays+=1;return new Promise((resolve,reject)=>{pending.push({resolve,reject});});},
 pause(){this.paused=true;this.emit('pause');}});
const video=asMedia(get('video'));video.tagName='VIDEO';
const document=new Element();document.documentElement={clientWidth:900};document.body=new Element();document.getElementById=get;
document.createElement=tag=>{created.push(tag);const el=new Element(tag.toUpperCase());return tag==='video'?asMedia(el):el;};document.createElementNS=(_,tag)=>new Element(tag);
document.querySelectorAll=()=>[];
const window=new Element(),requests=[];let timerID=0;
vm.runInNewContext(script,{document,window,fetch:(url,options)=>{requests.push({url,...JSON.parse(options.body)});return Promise.resolve({ok:true})},setInterval:()=>1,clearInterval(){},setTimeout:()=>++timerID,clearTimeout(){},setImmediate,console});
const turn=()=>new Promise(resolve=>setImmediate(resolve));
const lastState=()=>requests.at(-1);
const clock=()=>get('time').textContent;
(async()=>{
// 1) 换集后进度条仍然推进：单元素换源保留 timeupdate 监听。
video.currentTime=30;video.emit('timeupdate');
assert.equal(clock(),'00:30 / 04:00');
assert.equal(window.playerLoad('https://example.invalid/b.mp4',0),true);
await turn();
video.readyState=4;video.duration=240;video.currentTime=75;video.emit('timeupdate');
assert.equal(clock(),'01:15 / 04:00','进度/时间在换集后必须继续更新');
assert.equal(get('seek').value,String(Math.round(75/240*1000)));
// 2) 连续两次换源：只保留最后一次，且全程只有这一个 video 元素。
assert.equal(window.playerLoad('https://example.invalid/c.mp4',0),true);
assert.equal(window.playerLoad('https://example.invalid/d.mp4',0),true);
await turn();
assert.equal(video.src,'https://example.invalid/d.mp4');
assert.equal(created.filter(tag=>tag==='video').length,0,'不允许再创建第二个 video 元素');
assert.equal(Object.values(els).filter(el=>el.tagName==='VIDEO').length,1);
const generations=requests.map(r=>r.mediaGeneration).filter(v=>Number.isInteger(v));
assert.equal(generations.at(-1)>=2,true,'每次换源都要推进 mediaGeneration');
// 3) 被换源打断的旧加载（aborted）不算失败，也不会盖住新片源。
video.error={code:1};video.emit('error');
assert.equal(get('hint').textContent,'','换源打断不应提示加载失败');
video.error=null;video.emit('loadedmetadata');
await turn();
assert.equal(lastState().error,0);
// 4) load() 复位倍速后仍然是 2 倍速。
window.playerCommand('rate',2);
assert.equal(video.playbackRate,2);assert.equal(video.defaultPlaybackRate,2);
window.playerLoad('https://example.invalid/e.mp4',0);
assert.equal(video.playbackRate,2,'换源后必须保持 2 倍速');
assert.equal(video.defaultPlaybackRate,2);
// 即使引擎把 rate 直接重置成 1，也只能是转瞬，必须被拉回 2。
video.playbackRate=1;video.emit('ratechange');
assert.equal(video.playbackRate,2);assert.equal(video.defaultPlaybackRate,2);
// 5) 开始换集：立即静音暂停，旧 play() 成功也不能恢复出声。
window.playerCommand('play',0);await turn();
assert.equal(video.plays,1);
window.playerBeginSwitch();
assert.equal(video.muted,true,'换集时必须立即静音');
assert.equal(video.paused,true,'换集时必须立即暂停');
for(const p of pending)p.resolve();await turn();
assert.equal(video.paused,true,'旧 play() 完成后不能重新开始播放');
assert.equal(video.muted,true,'旧 play() 完成后不能恢复出声');
// 6) 新片源接手后恢复声音，并且仍然只有一个可发声的视频元素。
window.playerLoad('https://example.invalid/f.mp4',0);
await turn();
assert.equal(video.muted,false);assert.equal(video.volume,1);
assert.equal(Object.values(els).filter(el=>el.tagName==='VIDEO').length,1);
console.log('single-element progress, latest-wins load, aborted load, 2x across load, and begin-switch silence verified');
})().catch(error=>{console.error(error);process.exitCode=1});
`

const controlsHarness = `
const vm=require('node:vm'),assert=require('node:assert/strict');
const {script}=JSON.parse(require('node:fs').readFileSync(0,'utf8'));
class Element {
 constructor(tag='DIV'){this.tagName=tag;this.listeners={};this.children=[];this.attrs={};this.hidden=false;this.value='0';this.style={setProperty:(k,v)=>this.attrs[k]=v};this.dataset={};this.classes=new Set();this.classList={add:x=>this.classes.add(x),remove:x=>this.classes.delete(x)};}
 addEventListener(name,fn){(this.listeners[name]??=[]).push(fn);}
 emit(name,extra={}){const event={target:this,preventDefault(){this.prevented=true},...extra};for(const fn of this.listeners[name]||[])fn(event);return event;}
 appendChild(el){this.children.push(el);return el;}
 get firstChild(){return this.children[0]||null;}
 removeChild(el){this.children.splice(this.children.indexOf(el),1);}
 replaceChildren(){this.children=[];}
 setAttribute(k,v){this.attrs[k]=v;}
 getBoundingClientRect(){return this.rect||{left:0,top:0,width:0,height:0};}
 get offsetWidth(){return (this.rect&&this.rect.width)||0;}
 scrollIntoView(opts){this.scrolledTo=opts;}
 contains(el){return el===this||this.children.some(x=>x.contains(el));}
}
const els={};const get=id=>els[id]??=(new Element());
const video=get('video');Object.assign(video,{currentTime:0,duration:240,readyState:4,error:null,paused:true,ended:false,muted:false,volume:1,playbackRate:1,load(){},play(){this.paused=false;this.emit('play');return Promise.resolve()},pause(){this.paused=true;this.emit('pause')}});
const document=new Element();document.documentElement={clientWidth:900};document.body=new Element();document.getElementById=get;document.createElement=tag=>new Element(tag.toUpperCase());document.createElementNS=(_,tag)=>new Element(tag);
// 弹幕设置面板的按钮不是单个 id，模拟 querySelectorAll 以便验证交互。
const dmPos=()=>['mega','full','half','top'].map(pos=>{const b=new Element('BUTTON');b.dataset.pos=pos;return b;});
const dmSize=()=>['large','medium','small'].map(size=>{const b=new Element('BUTTON');b.dataset.size=size;return b;});
const dmSpeed=()=>['slow','medium','fast'].map(speed=>{const b=new Element('BUTTON');b.dataset.speed=speed;return b;});
const groups={'.dm-positions button':dmPos(),'.dm-sizes button':dmSize(),'.dm-speeds button':dmSpeed()};
document.querySelectorAll=sel=>groups[sel]||[];
const window=new Element(),requests=[],timers=new Map();let timerID=0;
vm.runInNewContext(script,{document,window,fetch:(url,options)=>{requests.push({url,...JSON.parse(options.body)});return Promise.resolve({ok:true})},setInterval:()=>1,clearInterval(){},setTimeout:(fn,delay)=>{timers.set(++timerID,{fn,delay});return timerID},clearTimeout:id=>timers.delete(id),console});
const action=()=>requests.filter(x=>x.url.startsWith('/action?')).at(-1);
const cfg={title:'<img src=x onerror=alert(1)>',episode:4,episodes:[{id:'ep-4',number:4},{id:'ep-5',number:5}],currentEpisode:'ep-4',qualities:['1080P','720P'],quality:'1080P',canPrevious:true,canNext:true,fullscreen:true};
assert.equal(window.playerConfigure(cfg),true);assert.equal(get('player-heading').hidden,false);assert.equal(get('player-title').textContent,cfg.title);assert.equal(get('episode-label').textContent,'第 4 集');assert.equal(get('fullscreen').attrs['aria-label'],'退出全屏');
get('next').emit('click');assert.equal(action().name,'next');assert.equal(action().value,'');
get('quality').emit('click');assert.equal(get('player-menu').hidden,false);get('player-menu').children[2].emit('click');assert.equal(action().name,'quality');assert.equal(action().value,'720P');
get('episode').emit('click');
// 选集打开时应把当前集滚到可视区，而不是从第一集开始。
const episodeGrid=get('player-menu').children[1];
assert.equal(episodeGrid.children[0].scrolledTo&&episodeGrid.children[0].scrolledTo.block,'center');
episodeGrid.children[1].emit('click');assert.equal(action().name,'episode');assert.equal(action().value,'ep-5');
get('rate').emit('click');get('player-menu').children[6].emit('click');assert.equal(video.playbackRate,2);
// 音量：悬停展开竖向滑杆，点图标静音，再点恢复。
get('volume-panel').rect={left:0,top:0,width:58,height:160};
get('volume-button').rect={left:820,top:600,width:30,height:34};
get('volume-button').emit('mouseenter');assert.equal(get('volume-panel').hidden,false);
const volLeft=parseFloat(get('volume-panel').style.left);
assert.equal(volLeft,820+15-58/2);assert.equal(get('volume-panel').style.right,'auto');
const volumeSlider=get('volume-slider');volumeSlider.value='35';volumeSlider.emit('input');
assert.equal(Math.round(video.volume*100),35);assert.equal(get('volume-value').textContent,'35');
get('volume-button').emit('click');assert.equal(video.muted,true);
get('volume-button').emit('click');assert.equal(video.muted,false);
// 指针离开按钮与滑杆后自动收起（150 毫秒宽限）。
get('volume-button').emit('mouseleave');
for(const [id,timer] of [...timers])if(timer.delay===150){timers.delete(id);timer.fn();}
assert.equal(get('volume-panel').hidden,true);
// 滚轮调音量：上滚加 5%，并弹出居中提示。
const beforeWheel=video.volume;
video.emit('wheel',{deltaY:-100,cancelable:true,preventDefault(){}});
assert.equal(Math.round(video.volume*100),Math.min(100,Math.round(beforeWheel*100)+5));
assert.equal(get('volume-toast').classes.has('show'),true);
assert.equal(get('volume-toast-value').textContent,Math.round(video.volume*100)+'%');
// 在弹层上滚动不应被当成调音量。
const beforePopup=video.volume;
window.emit('wheel',{deltaY:-100,cancelable:true,preventDefault(){}});
assert.equal(video.volume,beforePopup);
video.emit('wheel',{deltaY:100,cancelable:true,preventDefault(){}});
assert.equal(Math.round(video.volume*100),Math.round(beforeWheel*100));
// 菜单改为悬停展开，按钮底色不变。
get('player-menu').rect={left:0,top:0,width:216,height:200};
get('quality').rect={left:800,top:600,width:40,height:34};
get('quality').emit('mouseenter');assert.equal(get('player-menu').hidden,false);
// 弹层对齐到按钮中心，而不是一律贴右边。
const menuLeft=parseFloat(get('player-menu').style.left);
assert.equal(Number.isFinite(menuLeft),true);
// 视口宽 900：居中会超出右侧，因此被夹到 900-216-8=676。
assert.equal(menuLeft,676);assert.equal(get('player-menu').style.right,'auto');
get('quality').emit('mouseleave');
for(const [id,timer] of [...timers])if(timer.delay===160){timers.delete(id);timer.fn();}
assert.equal(get('player-menu').hidden,true);
get('seek').emit('pointerdown');get('seek').value='500';get('seek').emit('input');get('seek').emit('change');assert.equal(video.currentTime,120);
const key=k=>document.emit('keydown',{key:k,target:document.body});key('ArrowRight');assert.equal(video.currentTime,125);key('ArrowLeft');assert.equal(video.currentTime,120);key(' ');assert.equal(video.paused,false);
// Playback hides the chrome after three seconds, while the fullscreen exit remains available.
for(const [id,timer] of [...timers])if(timer.delay===3000){timers.delete(id);timer.fn();}
assert.equal(get('player-ui').classes.has('is-idle'),true);window.playerConfigure(cfg);assert.equal(get('player-ui').classes.has('is-idle'),true);
document.emit('pointermove');assert.equal(get('player-ui').classes.has('is-idle'),false);
get('quality').emit('click');assert.equal([...timers.values()].some(x=>x.delay===3000),false);
document.emit('keydown',{key:'Escape',target:new Element('INPUT')});assert.equal(action().name,'fullscreen');assert.equal(action().value,'false');
window.playerConfigure({fullscreen:false});const count=requests.length;key('Escape');assert.equal(requests.length,count);key('f');assert.equal(action().value,'true');
// 全屏按钮是切换：先进入全屏，再点一次应请求退出。
window.playerConfigure({fullscreen:true});get('fullscreen').emit('click');assert.equal(action().value,'false');
key(' ');assert.equal(video.paused,true);assert.equal([...timers.values()].some(x=>x.delay===3000),false);
// App-window fullscreen has its own action and never uses the OS-fullscreen action.
window.playerConfigure({fullscreen:false,windowFullscreen:false});assert.equal(get('player-heading').hidden,false);
get('window-fullscreen').emit('click');assert.equal(action().name,'windowFullscreen');assert.equal(action().value,'true');
window.playerConfigure({windowFullscreen:true});assert.equal(get('player-heading').hidden,true);
assert.equal(get('window-fullscreen').attrs['aria-label'],'退出窗口全屏');
get('window-fullscreen').emit('click');assert.equal(action().value,'false');
key('Escape');assert.equal(action().name,'windowFullscreen');assert.equal(action().value,'false');
window.playerConfigure({fullscreen:true});key('Escape');assert.equal(action().name,'fullscreen');
assert.equal(get('window-fullscreen').disabled,true);
window.playerConfigure({fullscreen:false,windowFullscreen:false});
// 弹幕设置：位置、字号、不透明度与开关都要回到应用侧保存。
window.playerConfigure({danmakuAvailable:true});
const layer=get('danmaku-layer');
groups['.dm-positions button'].find(b=>b.dataset.pos==='top').emit('click');
assert.equal(action().name,'danmakuPosition');assert.equal(action().value,'top');assert.equal(layer.dataset.pos,'top');
groups['.dm-sizes button'].find(b=>b.dataset.size==='small').emit('click');
assert.equal(action().name,'danmakuSize');assert.equal(action().value,'small');assert.equal(layer.dataset.size,'small');
// 弹幕速度：默认适中，点“慢”后回报应用并写入图层。
assert.equal(layer.dataset.speed,'medium');
groups['.dm-speeds button'].find(b=>b.dataset.speed==='slow').emit('click');
assert.equal(action().name,'danmakuSpeed');assert.equal(action().value,'slow');assert.equal(layer.dataset.speed,'slow');
const opacity=get('dm-opacity');opacity.value='40';opacity.emit('input');
assert.equal(action().name,'danmakuOpacity');assert.equal(action().value,'40');assert.equal(layer.style.opacity,'0.4');
const enabled=get('dm-enabled');enabled.checked=false;enabled.emit('change');
assert.equal(action().name,'danmaku');assert.equal(action().value,'false');assert.equal(layer.hidden,true);
// 弹幕面板改为悬停展开，指针移出面板即收起。
const panel=get('danmaku-panel');panel.hidden=true;
get('danmaku-button').emit('mouseenter');assert.equal(panel.hidden,false);
panel.emit('mouseleave');assert.equal(panel.hidden,true);
// 应用侧下发的设置要能重新渲染到面板上。
window.playerConfigure({danmakuEnabled:true,danmakuPosition:'mega',danmakuSize:'large',danmakuSpeed:'fast',danmakuOpacity:65});
assert.equal(layer.hidden,false);assert.equal(layer.dataset.pos,'mega');assert.equal(layer.dataset.size,'large');assert.equal(layer.dataset.speed,'fast');assert.equal(layer.style.opacity,'0.65');
assert.equal(groups['.dm-speeds button'].find(b=>b.dataset.speed==='fast').attrs['aria-pressed'],'true');
assert.equal(groups['.dm-positions button'].find(b=>b.dataset.pos==='mega').attrs['aria-pressed'],'true');
assert.equal(get('dm-opacity-value').textContent,'65%');
console.log('configuration, safe title, menu dispatch, seek, rate, volume, keyboard, idle and fullscreen exit verified');
// A source without danmaku must hide both its toggle and timeline.
window.playerConfigure({danmakuAvailable:false});
assert.equal(get('danmaku-button').hidden,true);assert.equal(layer.hidden,true);assert.equal(panel.hidden,true);
get('danmaku-button').emit('click');assert.equal(panel.hidden,true);
window.playerConfigure({danmakuAvailable:true});assert.equal(get('danmaku-button').hidden,false);
console.log('optional danmaku capability, panel position, size, opacity and switch verified');
`

const scriptHarness = `
const vm=require('node:vm'),assert=require('node:assert/strict');
const {script}=JSON.parse(require('node:fs').readFileSync(0,'utf8'));
const turn=()=>new Promise(setImmediate);
function setup(play,fetcher=()=>Promise.resolve({ok:true})){
 const video={currentTime:0,duration:24,readyState:2,error:null,paused:true,ended:false,muted:false,volume:1,playbackRate:1,addEventListener(){},load(){},play,pause(){this.paused=true;}};
 const window={addEventListener(){}};
 vm.runInNewContext(script,{document:{getElementById:id=>id==='video'?video:{textContent:''}},window,fetch:fetcher,setInterval:()=>1,clearInterval(){},console});
 return {video,command:window.playerCommand};
}
(async()=>{
 // A pause during an outstanding play must not trigger the muted autoplay fallback.
 let calls=0,rejectPlay;
 const a=setup(()=>{calls++;return new Promise((_,reject)=>rejectPlay=reject)});
 a.command('play',0);a.command('pause',0);
 rejectPlay(Object.assign(new Error('interrupted'),{name:'AbortError'}));await turn();
 assert.equal(calls,1);assert.equal(a.video.muted,false);
 // A real autoplay rejection still gets the supported muted fallback.
 calls=0;
 const b=setup(()=>++calls===1?Promise.reject(Object.assign(new Error('blocked'),{name:'NotAllowedError'})):Promise.resolve());
 b.command('play',0);await turn();assert.equal(calls,2);assert.equal(b.video.muted,true);
 // Unchanged paused state stops posting; an A -> B -> A change while B is in flight is retained.
 const requests=[];
 const c=setup(()=>Promise.resolve(),(_,options)=>new Promise(resolve=>requests.push({state:JSON.parse(options.body),resolve})));
 assert.equal(requests.length,1);requests[0].resolve({ok:true});await turn();
 c.command('pause',0);await turn();assert.equal(requests.length,1);
 c.command('seek',5);assert.equal(requests.length,2);
 c.command('seek',0);requests[1].resolve({ok:true});await turn();
 assert.equal(requests.length,3);assert.equal(requests[2].state.currentTime,0);
 requests[2].resolve({ok:true});await turn();
 console.log('pause intent, autoplay fallback, and ordered status reporting verified');
})().catch(error=>{console.error(error);process.exitCode=1});
`
