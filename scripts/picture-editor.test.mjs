import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// The choice of a variant of a set of a picture, read from picture-editor.js
// alone: its functions of choice touch no page.
const script=readFileSync(new URL('../internal/studio/ui/picture-editor.js',import.meta.url),'utf8');
const context=vm.createContext({});
vm.runInContext(script,context);
const {setImage,screenVariant}=context;

// The ladder of densities, written here from the prototype's list
// (snapshot obshchaya-kartinka/04), not read from the server.
const ladder=[85,100,125,150,175,200,300,400];
const names={bldpi:85,ldpi:100,aldpi:125,mdpi:150,amdpi:175,hdpi:200,xdpi:300,udpi:400};
const rows=(...densities)=>densities.map(name=>({density:names[name],file:`${name}.png`}));

// «Выключить набор вариантов» leaves the variant of the density nearest to
// ldpi on the ladder, the smaller of two as near - the five trials the
// inventory writes out of the prototype's eight (docs/block-00/obshchaya-
// kartinka.md, «Досъёмка 28.10.2026») and udpi+xdpi by the rule, the order
// of the rows not mattering. Defect caught: the larger of two as near
// taken, the distance counted another way, the first row taken.
test('a set turned off leaves the variant nearest to ldpi',()=>{
  for(const [given,left] of [[['ldpi','bldpi'],'ldpi'],[['mdpi','ldpi'],'ldpi'],[['hdpi','mdpi'],'mdpi'],[['bldpi','mdpi'],'bldpi'],
    [['aldpi','bldpi'],'bldpi'],[['udpi','xdpi'],'xdpi']]){
    for(const order of [given,[...given].reverse()]){
      assert.equal(setImage(rows(...order),ladder).density,names[left],`${order.join('+')}`);
    }
  }
  // A row of no picture is dropped by the editor and never chosen; of two
  // of one density the one of any direction and browser is taken.
  assert.equal(setImage([{density:100},{density:150,file:'m.png'}],ladder).file,'m.png');
  assert.equal(setImage([{density:100,file:'r.png',direction:'rtl'},{density:100,file:'a.png'}],ladder).file,'a.png');
  assert.equal(setImage([],ladder),null);
});

// A set shows the variant for the density of the screen: on a Retina screen
// (200) the prototype shows xdpi of udpi and xdpi, hdpi of hdpi and mdpi.
// Defect caught: the base or the first variant shown whatever the screen.
test('a set shows the variant for the screen',()=>{
  assert.equal(screenVariant(rows('udpi','xdpi'),200).density,300);
  assert.equal(screenVariant(rows('mdpi','hdpi'),200).density,200);
  assert.equal(screenVariant(rows('hdpi','ldpi'),100).density,100);
  assert.equal(screenVariant([{density:200,file:'b.png',webBrowser:'microsoft-internet-explorer-microsoft-edge-html'},{density:200,file:'a.png'}],200).file,'a.png');
});
