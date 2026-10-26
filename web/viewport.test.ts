import { test } from 'node:test';
import assert from 'node:assert/strict';
import { keptShort } from './viewport.ts';

const iphone = { width: 402, height: 874 };

test('a window shorter than the screen is short, in portrait and in landscape', () => {
	assert.equal(keptShort({ width: 402, height: 874 }, iphone), false);
	assert.equal(keptShort({ width: 402, height: 872 }, iphone), false);
	assert.equal(keptShort({ width: 402, height: 538 }, iphone), true);
	assert.equal(keptShort({ width: 874, height: 402 }, iphone), false);
	assert.equal(keptShort({ width: 874, height: 220 }, iphone), true);
});

test('a window of another width is never called short', () => {
	assert.equal(keptShort({ width: 320, height: 500 }, iphone), false);
});
