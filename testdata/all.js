#!/usr/bin/env node
import x from "react";
import "./dep";
export * as z from "./z";

/** Foo docs. */
@component({ selector: "x" })
export class Foo extends Bar {
  x = 1;
  static #z = 2;
  handle = () => 1;

  static {
    Foo.ready = true;
  }

  constructor(a) {
    super(a);
  }

  /** Value. */
  async get v() {
    return 1;
  }

  async *it() {}
}

// An arrow function.
export const f = async (a) => a;
function g() {}
export default function h() {}
var x2 = 1;
let x = 2;
const K = 3;
const Comp = () => <div>hi</div>;
module.exports = { g };
console.log("run");
