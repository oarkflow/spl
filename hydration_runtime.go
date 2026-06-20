package spl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Feature flags (bitmask) for tree-shaking
// ---------------------------------------------------------------------------

type jsFeature uint16

const (
	featCore         jsFeature = 1 << iota // always included
	featScope                              // always included (events/effects need it)
	featDebug                              // SPL.debug, debugRecord, getRenderStats
	featFocus                              // captureFocus, restoreFocus
	featBindings                           // patchBindings + helpers
	featEvents                             // patchEvents
	featModels                             // patchModels
	featAPI                                // patchAPI, apiParse, serializeForm
	featConditionals                       // patchConditionals
	featRefs                               // patchRefs (data-spl-ref element refs)
	featSchemaArrays                       // patchSchemaArrays + schemaArrayAction
	featForms                              // patchForms validation state
	featAll          = featCore | featScope | featDebug | featFocus | featBindings | featEvents | featModels | featAPI | featConditionals | featRefs | featSchemaArrays | featForms
)

// ---------------------------------------------------------------------------
// Runtime modules — raw JS source for each segment
// ---------------------------------------------------------------------------

// moduleCore: SPL namespace, signals, subscribe, signalRef, signalName, interpolate, resolveTemplate
const moduleCore = `var SPL=window.__SPL__=window.__SPL__||{};
SPL.compiledWithUnsafeEval=SPL.compiledWithUnsafeEval===true;
SPL.allowUnsafeEval=SPL.compiledWithUnsafeEval;
SPL.signals=SPL.signals||Object.create(null);
SPL.handlers=SPL.handlers||Object.create(null);
SPL.refs=SPL.refs||Object.create(null);
SPL.bindings=SPL.bindings||Object.create(null);
SPL.hooks=SPL.hooks||Object.create(null);
SPL._batchDepth=0;
SPL._batchQueue=SPL._batchQueue||[];
SPL._sameValue=function(a,b){
  if(typeof Object.is==='function'){return Object.is(a,b);}
  return a===b?(a!==0||1/a===1/b):a!==a&&b!==b;
};
SPL._notifyError=function(context,error){
  try{
    if(typeof SPL.onError==='function'){SPL.onError(context,error);}
  } catch(ignore){}
  if(typeof console!=='undefined' && console.error){
    try{console.error('[spl:error]', {context:context, error:error});}
    catch(ignore){console.error('[spl:error]', context, error);}
  }
};
SPL._notifyHook=function(name,args){
  var hook=SPL.hooks&&SPL.hooks[name];
  if(typeof hook==='function'){
    try{hook.apply(null,args||[]);}
    catch(err){SPL._notifyError('hook:'+name,err);}
  }
};
SPL._enqueue=function(fn){
  if(SPL._batchDepth){SPL._batchQueue.push(fn);return;}
  try{fn();}
  catch(err){SPL._notifyError('callback',err);}
};
SPL._flushBatch=function(){
  while(SPL._batchQueue.length){
    var fn=SPL._batchQueue.shift();
    try{fn();}
    catch(err){SPL._notifyError('batch',err);}
  }
};
SPL.batch=function(fn){
  if(typeof fn!=='function'){return;}
  if(SPL._batchDepth){fn();return;}
  SPL._batchDepth=1;
  try{fn();}
  finally{
    SPL._batchDepth=0;
    SPL._flushBatch();
  }
};
SPL.registerHandler=function(name,fn){
  if(typeof name!=='string' || !name){return;}
  if(typeof fn!=='function' && !Array.isArray(fn) && typeof fn!=='string'){return;}
  SPL.handlers[name]=fn;
};
SPL.ensureSignal=function(name,initial){
  name=String(name||'');
  if(!name){return {value:initial,subscribers:[]};}
  if(!SPL.signals[name]){SPL.signals[name]={value:initial,subscribers:[]};}
  return SPL.signals[name];
};
SPL.isSafePathSegment=function(part){
  return part!=='__proto__' && part!=='prototype' && part!=='constructor';
};
SPL.allowedPath=/^[A-Za-z_][A-Za-z0-9_]*(\.([A-Za-z_][A-Za-z0-9_]*|[0-9]+))*$/;
SPL.normalizePath=function(path){
  if(typeof path!=='string'){return '';}
  path=path.trim();
  if(!SPL.allowedPath.test(path)){return '';}
  var parts=path.split('.');
  for(var i=0;i<parts.length;i++){
    if(!SPL.isSafePathSegment(parts[i])){return '';}
  }
  return path;
};
SPL.read=function(name){return SPL.ensureSignal(name,null).value;};
SPL.write=function(name,value){
  name=String(name||'');
  if(!name){return;}
  var s=SPL.ensureSignal(name,value);
  if(SPL._sameValue(s.value,value)){return;}
  s.value=value;
  SPL._notifyHook('signal',[name,value]);
  var subscribers=s.subscribers.slice();
  subscribers.forEach(function(fn){
    if(typeof fn!=='function'){return;}
    SPL._enqueue(function(){
      try{fn(value);}
      catch(err){SPL._notifyError('signal',{name:name,error:err});}
    });
  });
};
SPL.subscribe=function(name,fn){
  name=String(name||'');
  if(typeof fn!=='function' || !name){return function(){};}
  var s=SPL.ensureSignal(name,null);
  for(var i=0;i<s.subscribers.length;i++){
    if(s.subscribers[i]===fn){
      return function(){SPL.unsubscribe(name,fn);};
    }
  }
  s.subscribers.push(fn);
  return function(){SPL.unsubscribe(name,fn);};
};
SPL.unsubscribe=function(name,fn){
  name=String(name||'');
  var s=SPL.signals&&SPL.signals[name];
  if(!s || typeof fn!=='function'){return false;}
  var i=s.subscribers.indexOf(fn);
  if(i<0){return false;}
  s.subscribers.splice(i,1);
  return true;
};
SPL.signalRef=function(name){
  name=String(name||'');
  if(!name){return null;}
  var signal=SPL.ensureSignal(name,null);
  if(signal.ref){return signal.ref;}
  signal.ref={
    __splSignalName:name,
    valueOf:function(){return SPL.read(name);},
    toString:function(){var value=SPL.read(name);return value==null?'':String(value);}
  };
  if(typeof Symbol!=='undefined' && Symbol.toPrimitive){
    signal.ref[Symbol.toPrimitive]=function(hint){
      var value=SPL.read(name);
      if(value==null){
        return hint==='number'?0:'';
      }
      return value;
    };
  }
  return signal.ref;
};
SPL.signalName=function(nameOrRef){
  if(typeof nameOrRef==='string' && Object.prototype.hasOwnProperty.call(SPL.signals,nameOrRef)){
    return nameOrRef;
  }
  if(nameOrRef && typeof nameOrRef==='object' && typeof nameOrRef.__splSignalName==='string'){
    return nameOrRef.__splSignalName;
  }
  return '';
};
SPL.escapeHTML=function(value){
  return String(value==null?'':value)
    .replace(/&/g,'&amp;')
    .replace(/</g,'&lt;')
    .replace(/>/g,'&gt;')
    .replace(/"/g,'&quot;')
    .replace(/'/g,'&#39;');
};
SPL.formatSignalValue=function(value){
  if(value===true){return 'true';}
  if(value===false){return 'false';}
  if(value==null){return '';}
  if(typeof value==='object'){return JSON.stringify(value,null,2);}
  return String(value);
};
SPL.interpolate=function(source){
  return String(source||'')
    .replace(/__SPL_RAW_SIGNAL__([A-Za-z0-9_.]+)__/g,function(_,path){
      return SPL.formatSignalValue(SPL.readTarget(path));
    })
    .replace(/__SPL_SIGNAL__([A-Za-z0-9_.]+)__/g,function(_,path){
      return SPL.escapeHTML(SPL.formatSignalValue(SPL.readTarget(path)));
    });
};
SPL.resolveTemplate=function(source){
  return String(source||'').replace(/\{\{\s*([A-Za-z0-9_][A-Za-z0-9_\.]*)\s*\}\}/g,function(_,path){
    var value=SPL.readTarget(path);
    if(value==null){return '';}
    if(typeof value==='object'){return JSON.stringify(value);}
    return String(value);
  });
};
SPL.readPath=function(path){
  path=SPL.normalizePath(path);
  if(!path){return undefined;}
  var dot=path.indexOf('.');
  if(dot<0){return SPL.read(path);}
  var rest=path.slice(dot+1);
  var obj=SPL.read(path.slice(0,dot));
  if(obj==null){return undefined;}
  var parts=rest.split('.');
  for(var i=0;i<parts.length;i++){
    if(obj==null){return undefined;}
    obj=obj[parts[i]];
  }
  return obj;
};
SPL.writePath=function(path,value){
  path=SPL.normalizePath(path);
  if(!path){return;}
  var dot=path.indexOf('.');
  if(dot<0){SPL.write(path,value);return;}
  var root=path.slice(0,dot);
  var rest=path.slice(dot+1);
  var obj=SPL.read(root);
  if(obj==null || typeof obj!=='object'){obj=Object.create(null);}
  var clone=JSON.parse(JSON.stringify(obj));
  var parts=rest.split('.');
  var cur=clone;
  for(var i=0;i<parts.length-1;i++){
    if(cur[parts[i]]==null || typeof cur[parts[i]]!=='object'){cur[parts[i]]=Object.create(null);}
    cur=cur[parts[i]];
  }
  cur[parts[parts.length-1]]=value;
  SPL.write(root,clone);
};`

// moduleScope: safe client action execution and event dispatch
const moduleScope = `SPL.normalizePath=function(path){
  if(typeof path!=='string'){return '';}
  path=path.trim();
  if(!SPL.allowedPath.test(path)){return '';}
  var parts=path.split('.');
  for(var i=0;i<parts.length;i++){
    if(!SPL.isSafePathSegment(parts[i])){return '';}
  }
  return path;
};
SPL.readTarget=function(path){
  path=SPL.normalizePath(path);
  if(!path){return undefined;}
  return path.indexOf('.')>=0?SPL.readPath(path):SPL.read(path);
};
SPL.writeTarget=function(path,value){
  path=SPL.normalizePath(path);
  if(!path){return value;}
  if(path.indexOf('.')>=0){SPL.writePath(path,value);return value;}
  SPL.write(path,value);
  return value;
};
SPL.parseCallArgs=function(raw){
  var args=[];var current='';var quote='';var escaped=false;var depth=0;
  for(var i=0;i<raw.length;i++){
    var ch=raw.charAt(i);
    if(quote){
      current+=ch;
      if(escaped){escaped=false;continue;}
      if(ch==='\\'){escaped=true;continue;}
      if(ch===quote){quote='';}
      continue;
    }
    if(ch==='"' || ch==="'"){quote=ch;current+=ch;continue;}
    if(ch==='('){depth++;current+=ch;continue;}
    if(ch===')'){if(depth>0){depth--;}current+=ch;continue;}
    if(ch===',' && depth===0){args.push(current.trim());current='';continue;}
    current+=ch;
  }
  var tail=current.trim();
  if(tail){args.push(tail);}
  return args;
};
SPL.parseDebounceExpression=function(spec){
  if(typeof spec!=='string'){return null;}
  spec=spec.trim();
  if(spec.indexOf('debounce(')!==0 || spec.charAt(spec.length-1)!==')'){return null;}
  var args=SPL.parseCallArgs(spec.slice('debounce('.length,-1));
  if(args.length!==2){return null;}
  var inner=args[0].trim();
  var delay=Number(args[1].trim());
  if(!inner || !isFinite(delay) || delay<0){return null;}
  return {delay:delay,spec:inner};
};
SPL.scheduleDebounced=function(payload,event,element){
  if(!payload){return undefined;}
  var delay=Number(payload.delay||0);
  if(!(delay>0)){
    if(Array.isArray(payload.actions)){return SPL.executeActions(payload.actions);}
    if(typeof payload.handler==='string'){return SPL.executeEvent(payload.handler,event,element);}
    if(typeof payload.spec==='string'){return SPL.executeEvent(payload.spec,event,element);}
    return undefined;
  }
  var key='__splDebounce_'+delay+'_';
  if(Array.isArray(payload.actions)){key+='actions_'+JSON.stringify(payload.actions);}
  else if(typeof payload.handler==='string'){key+='handler_'+payload.handler;}
  else{key+='spec_'+String(payload.spec||'');}
  if(element && element[key]){clearTimeout(element[key]);}
  var run=function(){
    if(Array.isArray(payload.actions)){return SPL.executeActions(payload.actions);}
    if(typeof payload.handler==='string'){return SPL.executeEvent(payload.handler,event,element);}
    if(typeof payload.spec==='string'){return SPL.executeEvent(payload.spec,event,element);}
    return undefined;
  };
  var timer=setTimeout(function(){
    if(element){delete element[key];}
    run();
  },delay);
  if(element){element[key]=timer;}
  return undefined;
};
SPL.executeActions=function(actions){
  if(!Array.isArray(actions)){return undefined;}
  actions.forEach(function(action){
    if(!action || typeof action.kind!=='string'){return;}
    var target=SPL.normalizePath(action.target||'');
    if(!target){return;}
    if(action.kind==='toggle'){
      SPL.writeTarget(target,!Boolean(SPL.readTarget(target)));
      return;
    }
    if(action.kind==='set'){
      SPL.writeTarget(target,action.value);
      return;
    }
    if(action.kind==='add'){
      SPL.writeTarget(target,Number(SPL.readTarget(target)||0)+Number(action.value||0));
      return;
    }
    if(action.kind==='sub'){
      SPL.writeTarget(target,Number(SPL.readTarget(target)||0)-Number(action.value||0));
    }
  });
  return undefined;
};
SPL.executeEvent=function(spec,event,element){
  if(typeof spec!=='string' || !spec){return undefined;}
  var debounceExpr=SPL.parseDebounceExpression(spec);
  if(debounceExpr){return SPL.scheduleDebounced(debounceExpr,event,element);}
  if(spec.charAt(0)==='{'){
    try{
      var payload=JSON.parse(spec);
      if(payload && typeof payload==='object'){
        if(payload.delay!=null || Array.isArray(payload.actions) || typeof payload.handler==='string' || typeof payload.spec==='string'){
          return SPL.scheduleDebounced(payload,event,element);
        }
      }
    } catch(err){
      if(typeof console!=='undefined' && console.error){console.error('[spl:event]', {spec:spec, error:err});}
      return undefined;
    }
  }
  if(spec.charAt(0)==='['){
    try{return SPL.executeActions(JSON.parse(spec));}
    catch(err){
      if(typeof console!=='undefined' && console.error){console.error('[spl:event]', {spec:spec, error:err});}
      return undefined;
    }
  }
  if(Object.prototype.hasOwnProperty.call(SPL.handlers,spec)){
    var handler=SPL.handlers[spec];
    if(typeof handler==='function'){return handler.call(element,event,element);}
    if(typeof handler==='string'){
      if(SPL.allowUnsafeEval && typeof SPL.executeLegacyEvent==='function'){return SPL.executeLegacyEvent(handler,event,element);}
      return undefined;
    }
    return SPL.executeActions(handler);
  }
  if(SPL.allowUnsafeEval && typeof SPL.executeLegacyEvent==='function'){
    return SPL.executeLegacyEvent(spec,event,element);
  }
  if(typeof console!=='undefined' && console.error){console.error('[spl:event]', {spec:spec, error:'unknown event handler'});}
  return undefined;
};`

// moduleLegacyEval: legacy expression execution for compatibility mode only
const moduleLegacyEval = `SPL.reservedNames={event:true,element:true,signal:true,toggle:true,setSignal:true,ref:true,select:true,selectAll:true,SPL:true,Math:true,Number:true,String:true,Boolean:true,Date:true,JSON:true,console:true,document:true,window:true,undefined:true,NaN:true,Infinity:true};
SPL.getScope=function(event,element,useRefs){
  var helpers={
    event:event,
    element:element,
    SPL:SPL,
    document:document,
    window:window,
    signal:function(name){return SPL.read(String(name));},
    toggle:function(nameOrRef){
      var name=SPL.signalName(nameOrRef);
      if(!name){return !Boolean(nameOrRef);}
      var next=!Boolean(SPL.read(name));
      SPL.write(name,next);
      return next;
    },
    setSignal:function(nameOrRef,value){
      var name=SPL.signalName(nameOrRef);
      if(!name){return value;}
      if(typeof value==='function'){
        var prev=SPL.read(name);
        value=value(prev);
      }
      SPL.write(name,value);
      return value;
    },
    ref:function(name){return SPL.refs[name]||null;},
    select:function(sel){return document.querySelector(sel);},
    selectAll:function(sel){return Array.from(document.querySelectorAll(sel));},
    Math:Math,
    Number:Number,
    String:String,
    Boolean:Boolean,
    Date:Date,
    JSON:JSON,
    console:console
  };
  return new Proxy(helpers,{
    has:function(target,key){
      if(typeof key==='symbol'){return key in target;}
      if(Object.prototype.hasOwnProperty.call(target,key)){return true;}
      if(Object.prototype.hasOwnProperty.call(SPL.handlers,key)){return true;}
      if(typeof globalThis!=='undefined' && key in globalThis){return false;}
      return Object.prototype.hasOwnProperty.call(SPL.signals,String(key));
    },
    get:function(target,key){
      if(typeof key==='symbol'){return target[key];}
      if(Object.prototype.hasOwnProperty.call(target,key)){return target[key];}
      if(Object.prototype.hasOwnProperty.call(SPL.handlers,key)){return SPL.handlers[key];}
      if(Object.prototype.hasOwnProperty.call(SPL.signals,String(key))){return SPL.read(String(key));}
      if(typeof globalThis!=='undefined' && key in globalThis){return globalThis[key];}
      if(useRefs){return SPL.signalRef(String(key));}
      return SPL.read(String(key));
    },
    set:function(target,key,value){
      if(typeof key==='symbol'){target[key]=value;return true;}
      if(Object.prototype.hasOwnProperty.call(target,key)){target[key]=value;return true;}
      SPL.write(String(key), value);
      return true;
    }
  });
};
SPL.statementCache=SPL.statementCache||{};
SPL.expressionCache=SPL.expressionCache||{};
SPL._cacheCount=SPL._cacheCount||{s:0,e:0};
SPL._evictCache=function(cache,countKey,max){
  if(SPL._cacheCount[countKey]<=max){return;}
  var keys=Object.keys(cache);
  var toRemove=keys.length>>2;
  for(var i=0;i<toRemove;i++){delete cache[keys[i]];}
  SPL._cacheCount[countKey]-=toRemove;
};
SPL.callHandler=function(result,event,element){
  if(typeof result!=='function'){return result;}
  return result.call(element,event,element,SPL.getScope(event,element,false));
};
SPL.runStatement=function(expr,event,element){
  var fn=SPL.statementCache[expr];
  if(!fn){
    try{fn=new Function('scope','event','element','with(scope){ '+expr+'; return undefined; }');}
    catch(e){if(typeof console!=='undefined'){console.error('[spl:stmt]',e);}return undefined;}
    SPL._evictCache(SPL.statementCache,'s',1000);
    SPL.statementCache[expr]=fn;
    SPL._cacheCount.s++;
  }
  return fn(SPL.getScope(event,element,true),event,element);
};
SPL.evalExpression=function(expr,event,element,useRefs){
  var fn=SPL.expressionCache[expr];
  if(!fn){
    fn=new Function('scope','event','element','with(scope){ return ('+expr+'); }');
    SPL._evictCache(SPL.expressionCache,'e',1000);
    SPL.expressionCache[expr]=fn;
    SPL._cacheCount.e++;
  }
  return fn(SPL.getScope(event,element,!!useRefs),event,element);
};
SPL.executeLegacyEvent=function(expr,event,element){
  try {
    return SPL.callHandler(SPL.evalExpression(expr,event,element,true),event,element);
  } catch (evalErr) {
    try {
      return SPL.callHandler(SPL.runStatement(expr,event,element),event,element);
    } catch (stmtErr) {
      if(typeof console!=='undefined' && console.error){
        console.error('[spl:event]', {expr:expr, evalError:evalErr, statementError:stmtErr});
      }
      throw stmtErr;
    }
  }
};
SPL.extractDeps=function(expr){
  var deps=[];var seen={};var inSingle=false;var inDouble=false;var prev='';
  for(var i=0;i<expr.length;i++){
    var ch=expr[i];
    if(ch==='"' && !inSingle && prev!=='\\'){inDouble=!inDouble;prev=ch;continue;}
    if(ch==="'" && !inDouble && prev!=='\\'){inSingle=!inSingle;prev=ch;continue;}
    if(inSingle||inDouble){prev=ch;continue;}
    if((ch>='A'&&ch<='Z')||(ch>='a'&&ch<='z')||ch==='_'){
      var start=i;
      i++;
      for(;i<expr.length;i++){
        var c=expr[i];
        if(!((c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')||c==='_')){break;}
      }
      var name=expr.slice(start,i);
      var before=start>0?expr[start-1]:'';
      if(before==='.'||SPL.reservedNames[name]){i--;prev=ch;continue;}
      if(!seen[name] && Object.prototype.hasOwnProperty.call(SPL.signals,name)){
        seen[name]=true;deps.push(name);
      }
      i--;prev=ch;continue;
    }
    prev=ch;
  }
  return deps;
};`

// moduleDebug: debug recording and render stats
const moduleDebug = `SPL.debug=SPL.debug||{enabled:true,totalRenders:0,views:Object.create(null),effects:Object.create(null),signals:Object.create(null)};
SPL.debugRecord=function(kind,key){
  if(!SPL.debug || !SPL.debug.enabled){return;}
  SPL.debug.totalRenders=(SPL.debug.totalRenders||0)+1;
  if(kind==='view'){
    SPL.debug.views[key]=(SPL.debug.views[key]||0)+1;
  } else if(kind==='effect'){
    SPL.debug.effects[key]=(SPL.debug.effects[key]||0)+1;
  }
  if(typeof console!=='undefined' && console.debug){
    console.debug('[spl:render]', {kind:kind, key:key, total:SPL.debug.totalRenders, views:SPL.debug.views, effects:SPL.debug.effects});
  }
};
SPL.getRenderStats=function(){
  return {
    totalRenders:SPL.debug.totalRenders||0,
    views:Object.assign({}, SPL.debug.views||{}),
    effects:Object.assign({}, SPL.debug.effects||{}),
    signals:Object.keys(SPL.signals||{}).reduce(function(acc,key){acc[key]=SPL.read(key);return acc;}, {})
  };
};`

// moduleDebugStub: no-op stubs when debug is disabled
const moduleDebugStub = `SPL.debugRecord=function(){};SPL.getRenderStats=function(){return {};};`

// moduleFocus: focus capture/restore for re-renders
const moduleFocus = `SPL.escapeSelectorValue=function(value){
  return String(value||'').replace(/\\/g,'\\\\').replace(/"/g,'\\"');
};
SPL.captureFocus=function(root){
  var active=document.activeElement;
  if(!active || !root || active===document.body){return null;}
  if(active!==root && !(root.contains && root.contains(active))){return null;}
  var selector='';
  if(active.id){
    selector='[id="'+SPL.escapeSelectorValue(active.id)+'"]';
  } else if(active.getAttribute){
    if(active.hasAttribute('data-spl-model')){
      selector='[data-spl-model="'+SPL.escapeSelectorValue(active.getAttribute('data-spl-model'))+'"]';
    } else if(active.hasAttribute('data-spl-bind-value')){
      selector='[data-spl-bind-value="'+SPL.escapeSelectorValue(active.getAttribute('data-spl-bind-value'))+'"]';
    } else if(active.name){
      selector='[name="'+SPL.escapeSelectorValue(active.name)+'"]';
    }
  }
  if(!selector){return null;}
  return {
    selector:selector,
    start:typeof active.selectionStart==='number'?active.selectionStart:null,
    end:typeof active.selectionEnd==='number'?active.selectionEnd:null,
    checked:typeof active.checked==='boolean'?active.checked:null
  };
};
SPL.restoreFocus=function(root,snapshot){
  if(!root || !snapshot || !snapshot.selector || !root.querySelector){return;}
  var next=null;
  try{next=root.querySelector(snapshot.selector);}
  catch(err){return;}
  if(!next || typeof next.focus!=='function'){return;}
  next.focus();
  if(snapshot.checked!==null && 'checked' in next){
    next.checked=snapshot.checked;
  }
  if(snapshot.start!==null && typeof next.setSelectionRange==='function'){
    var end=snapshot.end===null?snapshot.start:snapshot.end;
    next.setSelectionRange(snapshot.start,end);
  }
};`

// moduleFocusStub: no-op stubs when focus is not needed
const moduleFocusStub = `SPL.captureFocus=function(){return null;};SPL.restoreFocus=function(){};`

// moduleBindings: applyBinding, bindingEvent, readBindingValue, patchBindings
const moduleBindings = `SPL.normalizeBindingProp=function(prop){
  prop=String(prop||'');
  var lower=prop.toLowerCase();
  if(lower==='textcontent'){return 'textContent';}
  if(lower==='innerhtml' || lower==='html-unsafe'){return 'innerHTML';}
  if(lower==='html'){return 'html';}
  return prop;
};
SPL.isTrustedHTMLBinding=function(el){
  return el.getAttribute('data-spl-trusted-html')==='true' || el.hasAttribute('data-spl-bind-html-unsafe');
};
SPL.applyBinding=function(el,prop,value){
  prop=SPL.normalizeBindingProp(prop);
  if(value!=null && typeof value==='object'){value=JSON.stringify(value,null,2);}
  var text=value==null?'':String(value);
  if(prop==='html' || prop==='innerHTML'){
    if(SPL.isTrustedHTMLBinding(el)){
      el.innerHTML=text;
    } else {
      el.textContent=text;
    }
    return;
  }
  if(prop==='textContent'){el.textContent=text;return;}
  if(prop in el){el[prop]=value==null?'':value;return;}
  el.setAttribute(prop,text);
};
SPL.bindingEvent=function(el,prop){
  if(prop==='checked' || prop==='selectedIndex' || el.tagName==='SELECT'){return 'change';}
  return 'input';
};
SPL.readBindingValue=function(el,prop){
  prop=SPL.normalizeBindingProp(prop);
  if(prop==='textContent'){return el.textContent;}
  if(prop==='innerHTML' || prop==='html'){return el.innerHTML;}
  if(prop in el){return el[prop];}
  return el.getAttribute(prop);
};
SPL.patchBindings=function(root){
  var nodes=(root.matches && (root.matches('[data-spl-bind]') || Array.from(root.attributes||[]).some(function(attr){return attr.name.indexOf('data-spl-bind-')===0;})))?[root]:[];
  nodes=nodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-bind], [data-spl-bind-value], [data-spl-bind-checked], [data-spl-bind-textContent], [data-spl-bind-textcontent], [data-spl-bind-innerHTML], [data-spl-bind-innerhtml], [data-spl-bind-html-unsafe]') : []));
  nodes.forEach(function(el){
    var attrs=Array.from(el.attributes||[]);
    attrs.forEach(function(attr){
      if(attr.name==='data-spl-bind'){
        if(el.__splLegacyBound){return;}
        el.__splLegacyBound=true;
        var signalName=SPL.normalizePath(attr.value);
        if(!signalName){return;}
        var prop=SPL.normalizeBindingProp(el.getAttribute('data-spl-attr')||'textContent');
        var update=function(value){SPL.applyBinding(el,prop,value);};
        update(SPL.read(signalName));
        SPL.subscribe(signalName,update);
        if((prop==='value'||prop==='checked') && /^[A-Za-z_][A-Za-z0-9_]*$/.test(signalName)){
          el.addEventListener(SPL.bindingEvent(el,prop),function(){SPL.write(signalName,SPL.readBindingValue(el,prop));});
        }
        return;
      }
      if(attr.name.indexOf('data-spl-bind-')!==0){return;}
      var propName=SPL.normalizeBindingProp(attr.name.slice('data-spl-bind-'.length));
      var expr=attr.value;
      var path=SPL.normalizePath(expr);
      var bindKey='__splBind_'+propName+'_'+expr;
      if(el[bindKey]){return;}
      el[bindKey]=true;
      if(path){
        var rootSignal=path.split('.')[0];
        var updateExpr=function(){SPL.applyBinding(el,propName,SPL.readTarget(path));};
        updateExpr();
        SPL.subscribe(rootSignal,updateExpr);
        el.addEventListener(SPL.bindingEvent(el,propName),function(){SPL.writeTarget(path,SPL.readBindingValue(el,propName));});
        return;
      }
      if(!SPL.allowUnsafeEval || typeof SPL.evalExpression!=='function' || typeof SPL.extractDeps!=='function'){return;}
      var legacyUpdate=function(){SPL.applyBinding(el,propName,SPL.evalExpression(expr,null,el,false));};
      legacyUpdate();
      SPL.extractDeps(expr).forEach(function(dep){SPL.subscribe(dep,legacyUpdate);});
      if(/^[A-Za-z_][A-Za-z0-9_]*$/.test(expr)){
        el.addEventListener(SPL.bindingEvent(el,propName),function(){SPL.write(expr,SPL.readBindingValue(el,propName));});
      }
    });
  });
  var dynamicNodes=(root.matches && Array.from(root.attributes||[]).some(function(attr){return attr.name.indexOf('data-spl-class-')===0 || attr.name.indexOf('data-spl-style-')===0;}))?[root]:[];
  dynamicNodes=dynamicNodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('*') : []));
  dynamicNodes.forEach(function(el){
    Array.from(el.attributes||[]).forEach(function(attr){
      var isClass=attr.name.indexOf('data-spl-class-')===0;
      var isStyle=attr.name.indexOf('data-spl-style-')===0;
      if(!isClass && !isStyle){return;}
      var name=attr.name.slice(isClass?'data-spl-class-'.length:'data-spl-style-'.length);
      var expr=attr.value;
      var key='__splDyn_'+attr.name+'_'+expr;
      if(el[key]){return;}
      el[key]=true;
      var path=SPL.normalizePath(expr);
      var rootSignal=path?path.split('.')[0]:'';
      var read=function(){
        if(path){return SPL.readTarget(path);}
        if(SPL.allowUnsafeEval && typeof SPL.evalExpression==='function'){return SPL.evalExpression(expr,null,el,false);}
        return undefined;
      };
      var update=function(){
        var value=read();
        if(isClass){el.classList.toggle(name,Boolean(value));}
        else{el.style[name]=value==null?'':String(value);}
      };
      update();
      if(rootSignal){SPL.subscribe(rootSignal,update);}
      else if(SPL.allowUnsafeEval && typeof SPL.extractDeps==='function'){SPL.extractDeps(expr).forEach(function(dep){SPL.subscribe(dep,update);});}
    });
  });
};`

// moduleEvents: patchEvents
const moduleEvents = `SPL.patchEvents=function(root){
  var nodes=(root.matches && Array.from(root.attributes||[]).some(function(attr){return attr.name.indexOf('data-spl-on-')===0;}))?[root]:[];
  nodes=nodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('*') : []));
  nodes.forEach(function(el){
    Array.from(el.attributes||[]).forEach(function(attr){
      if(attr.name.indexOf('data-spl-on-')!==0 || attr.name.indexOf('-mods')===attr.name.length-5){return;}
      var eventName=attr.name.slice('data-spl-on-'.length);
      var expr=attr.value;
      var key='__splEvent_'+eventName+'_'+expr;
      if(el[key]){return;}
      el[key]=true;
      var mods=(el.getAttribute('data-spl-on-'+eventName+'-mods')||'').split(',').map(function(v){return v.trim();}).filter(Boolean);
      var options={};
      if(mods.indexOf('capture')>=0){options.capture=true;}
      if(mods.indexOf('once')>=0){options.once=true;}
      if(mods.indexOf('passive')>=0){options.passive=true;}
      el.addEventListener(eventName,function(event){
        if(mods.indexOf('prevent')>=0 && event && typeof event.preventDefault==='function'){event.preventDefault();}
        if(mods.indexOf('stop')>=0 && event && typeof event.stopPropagation==='function'){event.stopPropagation();}
        SPL.executeEvent(expr,event,el);
      },options);
    });
  });
};`

// moduleModels: patchModels (supports dot-path binding e.g. data-spl-model="form.name")
const moduleModels = `SPL.patchModels=function(root){
  var nodes=(root.matches && root.matches('[data-spl-model]'))?[root]:[];
  nodes=nodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-model]') : []));
  nodes.forEach(function(el){
    if(el.__splModelBound){return;}
    el.__splModelBound=true;
    var path=el.getAttribute('data-spl-model');
    var dot=path.indexOf('.');
    var signalName=dot<0?path:path.slice(0,dot);
    var isRadio=(el.type==='radio');
    var isCheckOrRadio=(el.type==='checkbox'||isRadio);
    var prop=isCheckOrRadio?'checked':'value';
    var update=function(){
      var value=dot<0?SPL.read(path):SPL.readPath(path);
      if(isRadio){el.checked=(String(value)===el.value);}
      else if(prop==='checked'){el.checked=Boolean(value);}
      else{el.value=value==null?'':String(value);}
    };
    update();
    var eventName=isCheckOrRadio?'change':'input';
    el.addEventListener(eventName,function(){
      var val=isRadio?el.value:(prop==='checked'?Boolean(el.checked):el.value);
      if(dot<0){SPL.write(path,val);}else{SPL.writePath(path,val);}
    });
    SPL.subscribe(signalName,update);
  });
};`

const moduleSchemaArrays = `SPL.schemaArrayBool=function(value, fallback){
  if(value==null || value===''){return fallback;}
  return value==='true' || value===true;
};
SPL.schemaArrayNumber=function(value, fallback){
  if(value==null || value===''){return fallback;}
  var n=Number(value);
  return isFinite(n)?n:fallback;
};
SPL.schemaArrayDefault=function(raw){
  try{return JSON.parse(raw||'null');}
  catch(err){return null;}
};
SPL.schemaArrayClone=function(value){
  if(value==null){return value;}
  return JSON.parse(JSON.stringify(value));
};
SPL.schemaArrayEnsureBounds=function(path,arr,min,max,defaultValue){
  var changed=false;
  if(!Array.isArray(arr)){arr=[];changed=true;}
  arr=SPL.schemaArrayClone(arr)||[];
  while(arr.length<min){
    arr.push(SPL.schemaArrayClone(defaultValue));
    changed=true;
  }
  if(isFinite(max) && arr.length>max){
    arr=arr.slice(0,max);
    changed=true;
  }
  if(changed){SPL.writeTarget(path,arr);}
  return arr;
};
SPL.schemaArrayRenderHTML=function(template,path,index,total,min){
  var levelMatch=template.match(/__SPL_ARRAY_INDEX_(\d+)__/);
  var level=levelMatch?levelMatch[1]:'0';
  var itemPath=path+'.'+index;
  var removeDisabled=total<=min?' disabled':'';
  var upDisabled=index<=0?' disabled':'';
  var downDisabled=index>=total-1?' disabled':'';
  return template
    .replace(new RegExp('__SPL_ARRAY_PARENT_PATH_'+level+'__','g'),path)
    .replace(new RegExp('__SPL_ARRAY_PATH_'+level+'__','g'),itemPath)
    .replace(new RegExp('__SPL_ARRAY_INDEX_'+level+'__','g'),String(index))
    .replace(new RegExp('__SPL_ARRAY_POSITION_'+level+'__','g'),String(index+1))
    .replace(new RegExp('__SPL_ARRAY_REMOVE_DISABLED_'+level+'__','g'),removeDisabled)
    .replace(new RegExp('__SPL_ARRAY_MOVE_UP_DISABLED_'+level+'__','g'),upDisabled)
    .replace(new RegExp('__SPL_ARRAY_MOVE_DOWN_DISABLED_'+level+'__','g'),downDisabled);
};
SPL.patchSchemaArrays=function(root){
  var hosts=(root.matches && root.matches('[data-spl-schema-array]'))?[root]:[];
  hosts=hosts.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-schema-array]') : []));
  hosts.forEach(function(host){
    if(host.__splArrayBinding){return;}
    host.__splArrayBinding=true;
    var path=host.getAttribute('data-spl-schema-array-path')||'';
    var rootSignal=path.split('.')[0];
    var templateEl=host.querySelector('template[data-spl-schema-array-template]');
    var itemsEl=host.querySelector('.spl-schema-array-items');
    if(!path || !rootSignal || !templateEl || !itemsEl){return;}
    var template=templateEl.innerHTML||templateEl.textContent||'';
    var min=SPL.schemaArrayNumber(host.getAttribute('data-spl-schema-array-min'),0);
    var max=SPL.schemaArrayNumber(host.getAttribute('data-spl-schema-array-max'),Infinity);
    var addable=SPL.schemaArrayBool(host.getAttribute('data-spl-schema-array-add'),true);
    var defaultValue=SPL.schemaArrayDefault(host.getAttribute('data-spl-schema-array-default'));
    var emptyMessage=host.getAttribute('data-spl-schema-array-empty-message')||'No items';
    var minMessage=host.getAttribute('data-spl-schema-array-min-message')||'Minimum items reached';
    var maxMessage=host.getAttribute('data-spl-schema-array-max-message')||'Maximum items reached';
    var messageEl=host.querySelector('.spl-schema-array-message');
    var render=function(){
      var arr=SPL.readTarget(path);
      arr=SPL.schemaArrayEnsureBounds(path,arr,min,max,defaultValue);
      var html=arr.map(function(_,index){return SPL.schemaArrayRenderHTML(template,path,index,arr.length,min);}).join('');
      var focusSnapshot=SPL.captureFocus(itemsEl);
      itemsEl.innerHTML=html || '<div class="spl-schema-empty">'+SPL.escapeHTML(emptyMessage)+'</div>';
      var addButtons=Array.from(host.querySelectorAll('[data-spl-schema-array-action="add"][data-spl-schema-array-path="'+path+'"]'));
      addButtons.forEach(function(addButton){addButton.disabled=!addable || arr.length>=max;});
      var removeButtons=Array.from(host.querySelectorAll('[data-spl-schema-array-action="remove"][data-spl-schema-array-path="'+path+'"]'));
      removeButtons.forEach(function(removeButton){removeButton.disabled=arr.length<=min;});
      if(messageEl){
        var message='';
        if(arr.length===0){message=emptyMessage;}
        else if(isFinite(max) && arr.length>=max){message=maxMessage;}
        else if(min>0 && arr.length<=min){message=minMessage;}
        messageEl.textContent=message;
        messageEl.hidden=!message;
      }
      SPL.patch(itemsEl);
      SPL.restoreFocus(itemsEl,focusSnapshot);
    };
    render();
    SPL.subscribe(rootSignal,render);
  });
};
SPL.schemaArrayAction=function(element){
  if(!element || !element.getAttribute){return;}
  var action=element.getAttribute('data-spl-schema-array-action')||'';
  var path=element.getAttribute('data-spl-schema-array-path')||'';
  if(!action || !path){return;}
  var host=(element.closest && element.closest('[data-spl-schema-array="true"][data-spl-schema-array-path="'+path+'"]')) || null;
  var min=SPL.schemaArrayNumber(host && host.getAttribute('data-spl-schema-array-min'),0);
  var max=SPL.schemaArrayNumber(host && host.getAttribute('data-spl-schema-array-max'),Infinity);
  var arr=SPL.readTarget(path);
  if(!Array.isArray(arr)){arr=[];}
  arr=SPL.schemaArrayClone(arr)||[];
  if(action==='add'){
    if(arr.length>=max){return;}
    var rawValue=element.getAttribute('data-spl-schema-array-value');
    arr.push(rawValue!=null?SPL.schemaArrayDefault(rawValue):SPL.schemaArrayDefault(host && host.getAttribute('data-spl-schema-array-default')));
  } else if(action==='remove'){
    if(arr.length<=min){return;}
    var removeIndex=Number(element.getAttribute('data-spl-schema-array-index'));
    if(!isFinite(removeIndex) || removeIndex<0 || removeIndex>=arr.length){return;}
    arr.splice(removeIndex,1);
  } else if(action==='move'){
    var index=Number(element.getAttribute('data-spl-schema-array-index'));
    var direction=Number(element.getAttribute('data-spl-schema-array-direction'));
    var next=index+direction;
    if(!isFinite(index) || !isFinite(next) || next<0 || next>=arr.length){return;}
    var tmp=arr[index];
    arr[index]=arr[next];
    arr[next]=tmp;
  } else {
    return;
  }
  SPL.writeTarget(path,arr);
};`

// moduleAPI: apiParse, serializeForm, patchAPI
const moduleAPI = `SPL.apiParse=function(res, mode){
  if(mode==='json'){return res.json();}
  if(mode==='text' || mode==='html'){return res.text();}
  var ct=(res.headers.get('content-type')||'').toLowerCase();
  if(ct.indexOf('application/json')>=0){return res.json();}
  return res.text();
};
SPL.assignPath=function(target,path,value){
  path=SPL.normalizePath(path);
  if(typeof target!=='object' || target==null || !path){return;}
  var parts=path.split('.');
  var cur=target;
  for(var i=0;i<parts.length-1;i++){
    var key=parts[i];
    if(!key){return;}
    if(cur[key]==null || typeof cur[key]!=='object' || Array.isArray(cur[key])){cur[key]=Object.create(null);}
    cur=cur[key];
  }
  var last=parts[parts.length-1];
  if(last){cur[last]=value;}
};
SPL.appendQueryValue=function(params,name,value){
  if(Array.isArray(value)){
    value.forEach(function(item){params.append(name,item==null?'':String(item));});
    return;
  }
  params.append(name,value==null?'':String(value));
};
SPL.serializeForm=function(form){
  var payload={};
  var radioGroups={};
  var checkboxGroups={};
  var checkboxModes={};
  if(!form){return payload;}
  Array.from(form.elements||[]).forEach(function(field){
    if(!field.name || field.disabled){return;}
    if(field.type==='radio'){
      if(field.checked){radioGroups[field.name]=field.value;}
      return;
    }
    if(field.type==='checkbox'){
      checkboxModes[field.name]=field.getAttribute('data-spl-checkbox-mode')||'auto';
      if(field.checked){
        if(!checkboxGroups[field.name]){checkboxGroups[field.name]=[];}
        checkboxGroups[field.name].push(field.value || true);
      }
      return;
    }
    SPL.assignPath(payload,field.name,field.value);
  });
  Object.keys(radioGroups).forEach(function(name){SPL.assignPath(payload,name,radioGroups[name]);});
  Object.keys(checkboxGroups).forEach(function(name){
    var values=checkboxGroups[name];
    var mode=checkboxModes[name];
    var value=mode==='boolean'?values.length>0:values.length===1?values[0]:values;
    SPL.assignPath(payload,name,value);
  });
  return payload;
};
SPL.apiURLWithQuery=function(url,data){
  var params;
  if(typeof URLSearchParams==='function'){
    params=new URLSearchParams();
  } else {
    params={append:function(name,value){this[name]=value;},toString:function(){var out=[];for(var k in this){if(Object.prototype.hasOwnProperty.call(this,k)){out.push(encodeURIComponent(k)+'='+encodeURIComponent(this[k]));}}return out.join('&');}};
  }
  Object.keys(data||{}).forEach(function(k){SPL.appendQueryValue(params,k,data[k]);});
  var qs=params.toString();
  if(!qs){return url;}
  return url+(url.indexOf('?')>=0?'&':'?')+qs;
};
SPL.patchAPI=function(root){
  var nodes=(root.matches && root.matches('[data-spl-api-url]'))?[root]:[];
  nodes=nodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-api-url]') : []));
  nodes.forEach(function(el){
    if(el.__splApiBound){return;}
    el.__splApiBound=true;
    var run=function(){
      var method=(el.getAttribute('data-spl-api-method')||'GET').toUpperCase();
      var url=SPL.resolveTemplate(el.getAttribute('data-spl-api-url')||'');
      var parseMode=(el.getAttribute('data-spl-api-parse')||'auto').toLowerCase();
      var target=el.getAttribute('data-spl-api-target')||'';
      var bodyTemplate=el.getAttribute('data-spl-api-body')||'';
      var resetSignals=(el.getAttribute('data-spl-api-reset')||'').split(',').map(function(v){return v.trim();}).filter(Boolean);
      var headers={};
      var body=null;
      var controller=null;
      if(typeof AbortController==='function'){
        if(el.__splApiController){el.__splApiController.abort();}
        controller=new AbortController();
        el.__splApiController=controller;
      }
      if(bodyTemplate!==''){
        body=SPL.resolveTemplate(bodyTemplate);
        headers['Content-Type']=el.getAttribute('data-spl-api-content-type')||'application/json';
      } else {
        var form = null;
        if(el.tagName==='FORM'){
          form = el;
        } else if(el.getAttribute('data-spl-api-form')==='closest' && typeof el.closest==='function') {
          form = el.closest('form');
        }
        if(form){
          var data=SPL.serializeForm(form);
          if(method==='GET' || method==='HEAD'){
            url=SPL.apiURLWithQuery(url,data);
          } else {
            headers['Content-Type']=el.getAttribute('data-spl-api-content-type')||'application/json';
            body=JSON.stringify(data);
          }
        }
      }
      var requestID=(el.__splApiRequestID||0)+1;
      el.__splApiRequestID=requestID;
      SPL._notifyHook('apiBefore',[el,{url:url,method:method,body:body}]);
      var init={method:method,headers:headers,body:body};
      if(controller){init.signal=controller.signal;}
      return fetch(url,init).then(function(res){
        if(requestID!==el.__splApiRequestID){return null;}
        return SPL.apiParse(res,parseMode).then(function(payload){
          if(requestID!==el.__splApiRequestID){return null;}
          if(!res.ok){
            throw {status:res.status,payload:payload};
          }
          if(target){SPL.writeTarget(target,payload);}
          resetSignals.forEach(function(name){SPL.writeTarget(name,'');});
          SPL._notifyHook('apiAfter',[el,{status:res.status,payload:payload}]);
          return payload;
        });
      }).catch(function(err){
        if(requestID!==el.__splApiRequestID){return;}
        if(err && err.status){
          SPL._notifyError('api',{status:err.status,payload:err.payload});
          if(target){SPL.writeTarget(target,'API error: HTTP '+err.status);}
          return;
        }
        if(String(err&&err.name)==='AbortError'){return;}
        SPL._notifyError('api',err);
        if(target){SPL.writeTarget(target,'API error: '+(err&&err.message?err.message:String(err)));}
      });
    };
    var eventName=(el.getAttribute('data-spl-api-event')||'click').toLowerCase();
    if(eventName==='load'){
      if(!el.__splApiLoaded){
        el.__splApiLoaded=true;
        run();
      }
      return;
    }
    el.addEventListener(eventName,function(ev){
      if(ev && typeof ev.preventDefault==='function' && (eventName==='submit' || el.tagName==='FORM' || (el.type==='submit' && typeof el.closest==='function' && el.closest('form')))){ev.preventDefault();}
      run();
    });
  });
};`

// moduleConditionals: patchConditionals
const moduleConditionals = `SPL.patchConditionals=function(root){
  var ifNodes=(root.matches && root.matches('[data-spl-if]'))?[root]:[];
  ifNodes=ifNodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-if]') : []));
  ifNodes.forEach(function(el){
    if(el.__splIfBound){return;}
    el.__splIfBound=true;
    var path=el.getAttribute('data-spl-if');
    var rootSignal=path.indexOf('.')>=0?path.split('.')[0]:path;
    var update=function(){
      el.style.display=Boolean(SPL.readTarget(path))?'':'none';
    };
    update();
    SPL.subscribe(rootSignal,update);
  });
  var elseNodes=(root.matches && root.matches('[data-spl-else]'))?[root]:[];
  elseNodes=elseNodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-else]') : []));
  elseNodes.forEach(function(el){
    if(el.__splElseBound){return;}
    el.__splElseBound=true;
    var path=el.getAttribute('data-spl-else');
    var rootSignal=path.indexOf('.')>=0?path.split('.')[0]:path;
    var update=function(){
      el.style.display=Boolean(SPL.readTarget(path))?'none':'';
    };
    update();
    SPL.subscribe(rootSignal,update);
  });
};`

const moduleRefs = `SPL.patchRefs=function(root){
  var nodes=(root.matches && root.matches('[data-spl-ref]'))?[root]:[];
  nodes=nodes.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('[data-spl-ref]') : []));
  nodes.forEach(function(el){
    var name=el.getAttribute('data-spl-ref');
    if(name){SPL.refs[name]=el;}
  });
};`

const moduleForms = `SPL.formFieldName=function(field){
  return field.getAttribute('data-spl-name') || field.name || field.id || '';
};
SPL.formState=function(form){
  var errors={};var touched={};var dirty={};var valid=true;
  Array.from(form.elements||[]).forEach(function(field){
    var name=SPL.formFieldName(field);
    if(!name || field.disabled){return;}
    if(field.__splTouched){touched[name]=true;}
    if(field.__splDirty){dirty[name]=true;}
    if(typeof field.checkValidity==='function' && !field.checkValidity()){
      valid=false;
      errors[name]=field.validationMessage || 'Invalid value';
    }
  });
  return {valid:valid,errors:errors,touched:touched,dirty:dirty};
};
SPL.patchForms=function(root){
  var forms=(root.matches && root.matches('form[data-spl-form-state]'))?[root]:[];
  forms=forms.concat(Array.from(root.querySelectorAll ? root.querySelectorAll('form[data-spl-form-state]') : []));
  forms.forEach(function(form){
    if(form.__splFormBound){return;}
    form.__splFormBound=true;
    var target=form.getAttribute('data-spl-form-state');
    var publish=function(){
      var state=SPL.formState(form);
      if(target){SPL.writeTarget(target,state);}
      Array.from(form.querySelectorAll('[data-spl-error-for]')).forEach(function(el){
        var name=el.getAttribute('data-spl-error-for');
        el.textContent=(state.errors&&state.errors[name])||'';
        el.hidden=!el.textContent;
      });
    };
    Array.from(form.elements||[]).forEach(function(field){
      field.addEventListener('blur',function(){field.__splTouched=true;publish();});
      field.addEventListener('input',function(){field.__splDirty=true;publish();});
      field.addEventListener('change',function(){field.__splDirty=true;publish();});
    });
    form.addEventListener('submit',function(event){
      Array.from(form.elements||[]).forEach(function(field){field.__splTouched=true;});
      publish();
      if(!form.checkValidity()){
        event.preventDefault();
      }
    });
    publish();
  });
};`

// splBootstrapJS reads inert JSON hydration payloads and wires up the page
// without inline bootstrap code.
const splBootstrapJS = `SPL.bootPayload=function(payload){
SPL.allowUnsafeEval=SPL.compiledWithUnsafeEval===true && payload.secure===false;
Object.keys(payload.signals||{}).forEach(function(name){
SPL.ensureSignal(name,payload.signals[name]);
});
Object.keys(payload.handlers||{}).forEach(function(name){
var actions=payload.handlers[name];
if(!actions){return;}
SPL.registerHandler(name,actions);
});
(payload.computed||[]).forEach(function(item){
if(!item || !item.Name){return;}
var render=function(){
var value;
if(SPL.allowUnsafeEval && typeof SPL.evalExpression==='function'){value=SPL.evalExpression(item.Expr,null,null,false);}
else{value=SPL.readTarget(item.Expr);}
SPL.write(item.Name,value);
};
render();
(item.Deps||[]).forEach(function(dep){SPL.subscribe(dep,render);});
});
SPL.patch(document);
(payload.effects||[]).forEach(function(effect){
var target=document.querySelector(effect.Selector);
if(!target){return;}
var render=function(){
var focusSnapshot=SPL.captureFocus(target);
target.innerHTML=SPL.interpolate(effect.Source);
SPL.patch(target);
SPL.restoreFocus(target,focusSnapshot);
SPL.debugRecord('effect', effect.Selector);
};
render();
(effect.Deps||[]).forEach(function(dep){SPL.subscribe(dep,render);});
});
(payload.views||[]).forEach(function(view){
var target=document.querySelector(view.Selector);
if(!target){return;}
var render=function(){
var focusSnapshot=SPL.captureFocus(target);
target.innerHTML=SPL.interpolate(view.Source);
SPL.patch(target);
SPL.restoreFocus(target,focusSnapshot);
SPL.debugRecord('view', view.Selector);
};
render();
(view.Deps||[]).forEach(function(dep){SPL.subscribe(dep,render);});
});
};
SPL.boot=function(root){
var scope=root||document;
var nodes=Array.from(scope.querySelectorAll?scope.querySelectorAll('script[data-spl-hydration][type="application/json"]'):[]);
nodes.forEach(function(node){
if(node.__splBooted){return;}
node.__splBooted=true;
try{SPL.bootPayload(JSON.parse(node.textContent||'{}'));}
catch(err){if(typeof console!=='undefined' && console.error){console.error('[spl:boot]',err);}}
});
};
if(typeof document!=='undefined'){
if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',function(){SPL.boot(document);});}
else{SPL.boot(document);}
}`

// ---------------------------------------------------------------------------
// JS Obfuscator — runs at init time, zero per-request cost
// ---------------------------------------------------------------------------

// genVarName generates short variable names: _a, _b, ..., _z, _aa, _ab, ...
func genVarName(n int) string {
	var name string
	for {
		name = string(rune('a'+(n%26))) + name
		n = n/26 - 1
		if n < 0 {
			break
		}
	}
	return "_" + name
}

// Internal SPL properties to mangle (not referenced in bootstrap or user code).
// Order is deterministic (sorted slice) so mangled names are stable.
var splInternalProps = []string{
	"apiParse",
	"applyBinding",
	"assignPath",
	"bindingEvent",
	"boot",
	"bootPayload",
	"captureFocus",
	"debugRecord",
	"escapeSelectorValue",
	"executeActions",
	"getRenderStats",
	"normalizePath",
	"patchAPI",
	"patchBindings",
	"patchConditionals",
	"patchEvents",
	"patchModels",
	"patchRefs",
	"patchSchemaArrays",
	"readBindingValue",
	"readTarget",
	"readPath",
	"restoreFocus",
	"schemaArrayBool",
	"schemaArrayClone",
	"schemaArrayDefault",
	"schemaArrayEnsureBounds",
	"schemaArrayNumber",
	"schemaArrayRenderHTML",
	"serializeForm",
	"signalName",
	"signalRef",
	"writeTarget",
	"writePath",
}

// buildPropMap creates the deterministic property mangling map
func buildPropMap() map[string]string {
	m := make(map[string]string, len(splInternalProps))
	for i, prop := range splInternalProps {
		m[prop] = genVarName(i)
	}
	return m
}

// propManglingMap is computed once at init
var propManglingMap map[string]string

func init() {
	propManglingMap = buildPropMap()
}

// encodeString converts a string literal to a char-code array decoding expression.
// Short strings (< 8 chars) use JSON quoting for safety.
func encodeString(s string) string {
	if len(s) < 8 {
		b, _ := json.Marshal(s)
		return string(b)
	}
	var codes []string
	for _, c := range s {
		codes = append(codes, fmt.Sprintf("%d", c))
	}
	return fmt.Sprintf("([%s].map(function(c){return String.fromCharCode(c)}).join(''))", strings.Join(codes, ","))
}

// obfuscateJS applies obfuscation to a JS source string:
//   - Property mangling: SPL.internalProp → SPL._xx
//   - String encoding: 'data-spl-xxx' → charCode array
//   - Comment stripping
//   - Redundant semicolon removal
//   - Whitespace minification
func obfuscateJS(src string) string {
	result := src

	// 1. Mangle SPL.propertyName → SPL._xx for internal properties.
	// This is safe because we only match the exact "SPL." prefix.
	for orig, mangled := range propManglingMap {
		result = strings.ReplaceAll(result, "SPL."+orig, "SPL."+mangled)
	}

	// 2. Encode data-spl-* string literals to charCode arrays.
	// Only encode strings in single quotes that contain 'data-spl-'.
	result = encodeDataSPLStrings(result)

	// 3. Remove comments (string-literal aware)
	result = stripJSComments(result)

	// 4. Remove redundant semicolons that contribute no meaning
	result = removeRedundantSemicolons(result)

	// 5. Minify: collapse whitespace
	result = minifyJS(result)

	return result
}

// dataSPLStringRe matches single-quoted strings containing 'data-spl-'
var dataSPLStringRe = regexp.MustCompile(`'(data-spl-[^']*)'`)

func encodeDataSPLStrings(src string) string {
	return dataSPLStringRe.ReplaceAllStringFunc(src, func(match string) string {
		inner := match[1 : len(match)-1]
		return encodeString(inner)
	})
}

// stripJSComments removes single-line (//) and block (/* */) comments from JS source.
// It is string-literal-aware to avoid breaking quoted content.
func stripJSComments(src string) string {
	var sb strings.Builder
	sb.Grow(len(src))

	inString := byte(0)
	lastWritten := byte(0)

	for i := 0; i < len(src); i++ {
		c := src[i]

		if inString != 0 {
			if c == inString && lastWritten != '\\' {
				inString = 0
			}
			sb.WriteByte(c)
			lastWritten = c
			continue
		}

		if c == '\'' || c == '"' || c == '`' {
			inString = c
			sb.WriteByte(c)
			lastWritten = c
			continue
		}

		// Single-line comment: //
		if c == '/' && i+1 < len(src) && src[i+1] == '/' {
			for i += 2; i < len(src); i++ {
				if src[i] == '\n' || src[i] == '\r' {
					break
				}
			}
			lastWritten = '\n'
			continue
		}

		// Block comment: /* */
		if c == '/' && i+1 < len(src) && src[i+1] == '*' {
			for i += 2; i < len(src); i++ {
				if src[i] == '*' && i+1 < len(src) && src[i+1] == '/' {
					i++ // skip the '/'
					break
				}
			}
			lastWritten = ' '
			continue
		}

		sb.WriteByte(c)
		lastWritten = c
	}

	// Collapse multiple consecutive newlines/whitespace into one
	cleaned := collapseRepeatedWhitespace(sb.String())
	return cleaned
}

// collapseRepeatedWhitespace reduces runs of whitespace to a single space,
// but preserves newlines (collapsing consecutive newlines to one).
func collapseRepeatedWhitespace(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	prevWS := false
	prevNL := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' || c == '\r' {
			if !prevNL {
				sb.WriteByte('\n')
			}
			prevNL = true
			prevWS = true
			continue
		}
		prevNL = false
		if c == ' ' || c == '\t' {
			if !prevWS {
				sb.WriteByte(' ')
			}
			prevWS = true
			continue
		}
		sb.WriteByte(c)
		prevWS = false
	}
	return sb.String()
}

// removeRedundantSemicolons removes semicolons that are unnecessary in JS:
//   - Semicolons before closing braces: ;} → }
//   - Trailing semicolons at end of output
func removeRedundantSemicolons(src string) string {
	inString := byte(0)
	lastWritten := byte(0)
	lastNonSpace := byte(0)

	for i := 0; i < len(src); i++ {
		if inString != 0 {
			if src[i] == inString && lastWritten != '\\' {
				inString = 0
			}
			lastWritten = src[i]
			continue
		}
		if src[i] == '\'' || src[i] == '"' || src[i] == '`' {
			inString = src[i]
			lastWritten = src[i]
			continue
		}
		lastWritten = src[i]
		lastNonSpace = src[i]
	}

	if lastNonSpace == ';' {
		src = src[:len(src)-1]
	}

	return strings.ReplaceAll(src, ";}", "}")
}

// minifyJS removes unnecessary whitespace from JS source.
// It's string-literal-aware to avoid breaking quoted content.
func minifyJS(src string) string {
	var sb strings.Builder
	sb.Grow(len(src))

	inString := byte(0) // 0, '\'' or '"'
	prevWasSpace := false
	lastWritten := byte(0)

	for i := 0; i < len(src); i++ {
		c := src[i]

		// Inside string literal: pass through unchanged
		if inString != 0 {
			if c == inString && lastWritten != '\\' {
				inString = 0
			}
			sb.WriteByte(c)
			lastWritten = c
			prevWasSpace = false
			continue
		}

		// Entering string literal
		if c == '\'' || c == '"' || c == '`' {
			if prevWasSpace && sb.Len() > 0 {
				if isWordChar(lastWritten) {
					sb.WriteByte(' ')
					lastWritten = ' '
				}
			}
			prevWasSpace = false
			inString = c
			sb.WriteByte(c)
			lastWritten = c
			continue
		}

		// Whitespace: collapse to at most one space
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			prevWasSpace = true
			continue
		}

		// Non-whitespace: emit deferred space if needed
		if prevWasSpace && sb.Len() > 0 {
			if needsSpaceBetween(lastWritten, c) {
				sb.WriteByte(' ')
				lastWritten = ' '
			}
			prevWasSpace = false
		}

		sb.WriteByte(c)
		lastWritten = c
	}
	return sb.String()
}

func needsSpaceBetween(a, b byte) bool {
	return isWordChar(a) && isWordChar(b)
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$'
}

// ---------------------------------------------------------------------------
// Module assembly + tree-shaking
// ---------------------------------------------------------------------------

// buildPatchFunction generates the SPL.patch function based on included features
func buildPatchFunction(features jsFeature) string {
	var calls []string
	if features&featSchemaArrays != 0 {
		calls = append(calls, "SPL.patchSchemaArrays(scope);")
	}
	if features&featBindings != 0 {
		calls = append(calls, "SPL.patchBindings(scope);")
	}
	if features&featModels != 0 {
		calls = append(calls, "SPL.patchModels(scope);")
	}
	if features&featEvents != 0 {
		calls = append(calls, "SPL.patchEvents(scope);")
	}
	if features&featAPI != 0 {
		calls = append(calls, "SPL.patchAPI(scope);")
	}
	if features&featConditionals != 0 {
		calls = append(calls, "SPL.patchConditionals(scope);")
	}
	if features&featRefs != 0 {
		calls = append(calls, "SPL.patchRefs(scope);")
	}
	if features&featForms != 0 {
		calls = append(calls, "SPL.patchForms(scope);")
	}
	if len(calls) == 0 {
		return "SPL.patch=function(){};"
	}
	return "SPL.patch=function(root){var scope=root||document;" + strings.Join(calls, "") + "};"
}

// assembleRuntime builds the full runtime JS for the given feature set
func assembleRuntime(features jsFeature, disableDebug bool, secureMode bool) string {
	var sb strings.Builder

	// Core + Scope are always included
	sb.WriteString(moduleCore)
	sb.WriteString("\n")
	sb.WriteString(moduleScope)
	sb.WriteString("\n")
	if !secureMode {
		sb.WriteString(moduleLegacyEval)
		sb.WriteString("\n")
	}

	// Debug
	if disableDebug {
		sb.WriteString(moduleDebugStub)
	} else {
		sb.WriteString(moduleDebug)
	}
	sb.WriteString("\n")

	// Focus (needed when effects/views exist)
	if features&featFocus != 0 {
		sb.WriteString(moduleFocus)
	} else {
		sb.WriteString(moduleFocusStub)
	}
	sb.WriteString("\n")

	// Optional modules
	if features&featBindings != 0 {
		sb.WriteString(moduleBindings)
		sb.WriteString("\n")
	}
	if features&featEvents != 0 {
		sb.WriteString(moduleEvents)
		sb.WriteString("\n")
	}
	if features&featModels != 0 {
		sb.WriteString(moduleModels)
		sb.WriteString("\n")
	}
	if features&featSchemaArrays != 0 {
		sb.WriteString(moduleSchemaArrays)
		sb.WriteString("\n")
	}
	if features&featAPI != 0 {
		sb.WriteString(moduleAPI)
		sb.WriteString("\n")
	}
	if features&featConditionals != 0 {
		sb.WriteString(moduleConditionals)
		sb.WriteString("\n")
	}
	if features&featRefs != 0 {
		sb.WriteString(moduleRefs)
		sb.WriteString("\n")
	}
	if features&featForms != 0 {
		sb.WriteString(moduleForms)
		sb.WriteString("\n")
	}

	// Patch function (adapted to included modules)
	sb.WriteString(buildPatchFunction(features))
	sb.WriteString("\n")
	sb.WriteString(splBootstrapJS)

	return sb.String()
}

// detectFeatures scans rendered HTML and hydration sources for feature usage
func detectFeatures(renderedHTML string, effects []hydrationEffect, views []hydrationView) jsFeature {
	features := featCore | featScope // always needed

	// Effects or views exist → need focus
	if len(effects) > 0 || len(views) > 0 {
		features |= featFocus
	}

	// Scan each source individually to avoid allocating a joined string
	sources := make([]string, 0, 1+len(effects)+len(views))
	sources = append(sources, renderedHTML)
	for _, e := range effects {
		sources = append(sources, e.Source)
	}
	for _, v := range views {
		sources = append(sources, v.Source)
	}

	for _, src := range sources {
		if features&featBindings == 0 && (strings.Contains(src, "data-spl-bind") || strings.Contains(src, "data-spl-class-") || strings.Contains(src, "data-spl-style-")) {
			features |= featBindings
		}
		if features&featEvents == 0 && strings.Contains(src, "data-spl-on-") {
			features |= featEvents
		}
		if features&featModels == 0 && strings.Contains(src, "data-spl-model") {
			features |= featModels
		}
		if features&featSchemaArrays == 0 && strings.Contains(src, "data-spl-schema-array") {
			features |= featSchemaArrays | featModels | featEvents | featFocus
		}
		if features&featAPI == 0 && strings.Contains(src, "data-spl-api-") {
			features |= featAPI
		}
		if features&featConditionals == 0 && (strings.Contains(src, "data-spl-if") || strings.Contains(src, "data-spl-else")) {
			features |= featConditionals
		}
		if features&featRefs == 0 && strings.Contains(src, "data-spl-ref") {
			features |= featRefs
		}
		if features&featForms == 0 && strings.Contains(src, "data-spl-form-state") {
			features |= featForms
		}
	}

	return features
}

// ---------------------------------------------------------------------------
// Obfuscated runtime cache
// ---------------------------------------------------------------------------

func getObfuscatedFull(disableDebug, secureMode bool) string {
	return getObfuscatedForFeatures(featAll, disableDebug, secureMode)
}

// moduleCache caches obfuscated modules by feature bitmask
var moduleCache = struct {
	sync.RWMutex
	cache map[jsFeature]string
}{cache: make(map[jsFeature]string)}

func getObfuscatedForFeatures(features jsFeature, disableDebug, secureMode bool) string {
	key := features
	if disableDebug {
		key |= 1 << 15
	}
	if secureMode {
		key |= 1 << 14
	}

	moduleCache.RLock()
	cached, ok := moduleCache.cache[key]
	moduleCache.RUnlock()
	if ok {
		return cached
	}

	raw := assembleRuntime(features, disableDebug, secureMode)
	obfuscated := obfuscateJS(raw)

	moduleCache.Lock()
	moduleCache.cache[key] = obfuscated
	moduleCache.Unlock()

	return obfuscated
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// RuntimeJS returns the fully obfuscated SPL hydration runtime JavaScript
// with all features included. Serve this as a static .js file and set
// Engine.HydrationRuntimeURL to enable browser caching across pages.
func (e *Engine) RuntimeJS() string {
	return getObfuscatedForFeatures(featAll, e.DisableDebug, e.SecureMode)
}

// RuntimeJSRaw returns the unobfuscated, minified SPL hydration runtime.
// Useful for debugging.
func (e *Engine) RuntimeJSRaw() string {
	raw := assembleRuntime(featAll, e.DisableDebug, e.SecureMode)
	return minifyJS(raw)
}

// ClearRuntimeCache clears the in-memory cache of obfuscated runtime modules.
// Call this if you change runtime configuration (e.g. SecureMode) at runtime
// and need previously cached variants to be rebuilt.
func ClearRuntimeCache() {
	moduleCache.Lock()
	moduleCache.cache = make(map[jsFeature]string)
	moduleCache.Unlock()
}
