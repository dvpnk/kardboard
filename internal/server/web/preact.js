// The one place that imports Preact, its hooks and htm from vendor/ (see
// vendor/README.md). Components import from here. html is htm bound to
// Preact: tagged templates instead of JSX, so the page needs no build step.

import { h, render, Fragment } from "./vendor/preact.mjs";
import { useState, useEffect, useRef, useReducer } from "./vendor/hooks.mjs";
import htm from "./vendor/htm.module.js";

export { h, render, Fragment, useState, useEffect, useRef, useReducer };
export const html = htm.bind(h);

// cls joins the class names that apply, skipping false, null and "".
export const cls = (...names) => names.filter(Boolean).join(" ");
