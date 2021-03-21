/**
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

function myersDiff(from, to) {
  var max = from.length + to.length;
  if (max == 0) {
    return []
  }
  v = new Array(2 * max + 1);
  v[max + 1] = { 'x': 0, 'path': '' };
  for (var d = 0; d <= max; d++) {
    for (var k = -d; k <= d; k = k + 2) {
      var x;
      var path;
      if (k == -d || (k != d && v[k+max-1].x < v[k+max+1].x)) {
        x = v[k+max+1].x;
        path = v[k+max+1].path + '-';
      } else {
        x = v[k+max-1].x + 1;
        path = v[k+max-1].path + '+';
      }
      var y = x - k;
      var same = '';
      while ((x < to.length) && (y < from.length) && (to[x] == from[y])) {
        x++;
        y++;
        same = same + '=';
      }
      if ((x >= to.length) && (y >= from.length)) {
        var edits = path.slice(1) + same;
        var diff = [];
        var i = 0;
        var fi = 0;
        var ti = 0;
        while (i < edits.length) {
          for (var j = i+1; j < edits.length && edits[i] == edits[j]; j++) {}
          if (edits[i] == '=') {
            diff.push('=' + from.slice(fi, fi + j - i));
            fi = fi + j - i;
            ti = ti + j - i;
          } else if (edits[i] == '-') {
            diff.push('-' + from.slice(fi, fi + j - i));
            fi = fi + j - i;
          } else if (edits[i] == '+') {
            diff.push('+' + to.slice(ti, ti + j - i));
            ti = ti + j - i;
          } else {
            console.log("unexpected edit type " + edits[i]);
          }
          i = j;
        }
        return diff;
      }
      v[k+max] = { 'x': x, 'path': path + same };
    }
  }
  return "Something went wrong!";
}

// cleanDiffs processes an array of diff strings (each string prefixed
// with either '=' or '<type>' where <type> is either '+' or '-').  If
// the string is '=' then the strings on either side are <type>, and if
// the string is short, this will add noise to the diffs.  For example,
// if we started with "transformation" but changed it to "apple or pear",
// left diffs would be ["-tr", "=a", "-nsf", "=or", "-m", "=a", "-tion"],
// and right diffs would be ["=a", "+pple ", "=or", "+ pe", "=a", "+r"],
// which are fairly noisy.  After cleaning, the diffs would display as
// ["-transformation"], and ["+apple or pear"] (note: the cleaned diffs
// will still have the short segments, but they will display as if we
// had the larger diffs.
function cleanDiffs(diffs, type) {
  const kMinSameLength = 5;
  return diffs.map(function (x) {
    if (x[0] == '=' && x.length < kMinSameLength) {
      return type + x.slice(1);
    }
    return x;
  });
}

// fineDiffs produces the character level right/left diffs, where each
// diff is an array of diff strings (a diff string starts with the diff
// type, either '=', '+', or '-').  The 'left' diff string will only have
// types '-' or '=', while right diff strings will only have '+' or '='.
function fineDiffs(from, to) {
  let cdiffs = myersDiff(from, to);
  let left = cleanDiffs(cdiffs.filter(x => x[0] != '+'), '-');
  let right = cleanDiffs(cdiffs.filter(x => x[0] != '-'), '+');
  return [left, right];
}

function diffLines(file, diff, pairs, comments) {
  if (!diff) {
    bodies = [{
      loading: true,
      added: pairs.Added,
      deleted: pairs.Deleted,
      changed: pairs.Changed
    }];
    return bodies;
  }

  let lines = [];
  let split = diff.split('\n');
  let leftline = 0;
  let leftlines = {};
  let rightline = 0;
  let rightlines = {};
  let change = 0;
  // Convert line diffs collapsing add/delete sequences into edits.
  for (let i = 0; i < split.length; i++) {
    let type = split[i][0];
    let line = split[i].replace('\r', '');
    if (line.length === 0) {
      line = ' ';
    }
    if (type == '=') {
      change = 0;
      leftlines[leftline] = lines.length;
      rightlines[rightline] = lines.length;
      leftline++;
      rightline++;
      lines.push({
        'leftno': leftline,
        'rightno': rightline,
        'lefttype': '=',
        'righttype': '=',
        'left': [line],
        'leftraw': line,
        'right': [line],
        'rightraw': line,
        'comments': [],
      });
    } else if (type == '-') {
      let j = leftline;
      leftline++;
      if (change > 0) {
        var idx = lines.length - change;
        leftlines[j] = idx;
        const to = lines[idx]['right'][0].slice(1);
        const [left, right] = fineDiffs(line.slice(1), to);
        lines[idx]['right'] = right;
        lines[idx]['leftno'] = leftline;
        lines[idx]['left'] = left;
        lines[idx]['lefttype'] = '-';
      } else {
        leftlines[j] = lines.length;
        lines.push({
          'leftno': leftline,
          'rightno': ' ',
          'leftraw': line,
          'left': [line],
          'right': ' ',
          'rightraw': null,
          'lefttype': '-',
          'righttype': '.-',
          'comments': [],
        });
      }
      change--;
    } else if (type == '+') {
      let j = rightline;
      rightline++;
      if (change < 0) {
        var idx = lines.length + change;
        rightlines[j] = idx;
        const from = lines[idx]['left'][0].slice(1);
        const [left, right] = fineDiffs(from, line.slice(1));
        lines[idx]['left'] = left;
        lines[idx]['rightno'] = rightline;
        lines[idx]['right'] = right;
        lines[idx]['righttype'] = '+';
      } else {
        rightlines[j] = lines.length;
        lines.push({
          'leftno': ' ',
          'rightno': rightline,
          'left': ' ',
          'leftraw': null,
          'right': [line],
          'rightraw': line,
          'lefttype': '.+',
          'righttype': '+',
          'comments': [],
        });
      }
      change++;
    } else {
      console.log("unexpected type " + type);
    }
  }

  // Insert inline comments.
  if (!comments) {
    comments = [];
  }
  for (let ci of comments) {
    if (ci.comment.context.leftLine != 0) {
      const i = matchContext(ci.comment.context, lines, leftlines, 'left');
      if (i >= 0 && i < lines.length) {
        let j = 0;
        for (; j < lines[i].comments.length; j++) {
          if (!lines[i].comments[j].left) {
            lines[i].comments[j].left = ci;
            break;
          }
        }
        if (j >= lines[i].comments.length) {
          lines[i].comments.push({
            left: ci,
            right: null,
          });
        }
        highlight(lines, i, ci.comment, 'left')
      }
    }
    if (ci.comment.context.rightLine != 0) {
      const i = matchContext(ci.comment.context, lines, rightlines, 'right');
      if (i >= 0 && i < lines.length) {
        let j = 0;
        for (; j < lines[i].comments.length; j++) {
          if (!lines[i].comments[j].right) {
            lines[i].comments[j].right = ci;
            break;
          }
        }
        if (j >= lines[i].comments.length) {
          lines[i].comments.push({
            left: null,
            right: ci,
          });
        }
        highlight(lines, i, ci.comment, 'right')
      }
    }
  }

  return lines;
}

// matchContext attempts to find the line number associated with a comment.
//
// parameters:
//   context: the Swarm context object from the comment
//   lines: the processed/combined lines of the file pair
//   index: map from left side/right side line numbers to combined lines
//   side: 'left' or 'right' indicating which 'side' the comment belongs to
// returns: the index into 'lines' where the comment should be attached
//
// Since the content may change from one version to another, there's no
// guarantee that the line number recorded with the comment still corresponds
// with the context of the original comment.  This function first checks if
// the code at the original location is unchanged, and if not, searches forward
// and back from that point to find the closest location where the context
// does match.  If no match is found, the original line is used.
// TODO: explore ways to improve context matching:
//   * match whole context instead of just last line
//   * consider partial matches instead of just exact matches
//   * if context matching improves sufficiently, it may be better to not render
//     unmatched inline comments (they'd still be shown in the comment pane)
//     vs. rendering inline comments in the wrong place.
function matchContext(context, lines, index, side) {
  // Lookup the line where we expect the comment to go from the comment context.
  let sideLine = context[`${side}Line`]-1;
  if (sideLine >= index.length) {
    // Expected line is out of range, so cap it to the end of the file.
    sideLine = index.length-1;
  }
  const i = index[sideLine];    // expected index for comment

  const raw = `${side}raw`;
  const content = context.content || [];
  const line = content.length > 0 ? content[content.length-1] : null;

  if (!line || !lines[i] || lines[i][`${side}raw`] == line) {
    // Either no content associated with context, or the content matches the
    // current version.  Nothing to do, return the index.
    return i;
  }

  // Search forward and backward from the expected line.
  let step = 1;
  const limit = lines.length - i - 1;
  while (true) {
    if ((step > i) && (step >= limit)) {
      // We've examined every line.  Give up.
      break;
    }
    if ((step <= i) && lines[i-step][raw] &&
        lines[i-step][raw].startsWith(line)) {
      return i-step;
    }
    if ((step < limit) && lines[i+step][raw] &&
        lines[i+step][raw].startsWith(line)) {
      return i+step;
    }
    step++;
  }
  return i;
}

// Recursively annotates the resolved and read status of the comment and
// it's children.
function annotateComments(ci) {
  let resolved = true;
  const children = ci.children || [];
  let allRead = (children.length > 0) ? true : ci.read;
  for (const child of children) {
    annotateComments(child);
    resolved = resolved && child.resolved && child.comment.id >= 0;
    allRead = allRead && child.allRead;
  }
  if (children.length > 0) {
    ci.resolved = resolved;
  } else {
    ci.resolved = (ci.comment.flags || []).indexOf('resolved') >= 0;
  }
  ci.allRead = allRead;
}

// Annotates lines with data to highlight comment selections.
function highlight(lines, i, comment, side) {
  const content = comment.context ? comment.context.content : null;
  if (!content || content.length < 1) {
    // No content, nothing to do.
    return;
  }

  const flags = comment.flags || [];
  let startOffset = null;
  let idx = flags.findIndex((x) => x.startsWith('context-start-offset='));
  if (idx >= 0) {
    startOffset = parseInt(flags[idx].substring(flags[idx].indexOf('=')+1));
  }
  if (!startOffset) {
    startOffset = 0;
  }

  let endOffset = null;
  idx = flags.findIndex((x) => x.startsWith('context-end-offset='));
  if (idx >= 0) {
    endOffset = parseInt(flags[idx].substring(flags[idx].indexOf('=')+1));
  }
  if (!endOffset) {
    endOffset = content[content.length-1].length;
  }

  const raw = `${side}raw`;
  const dst = `${side}Highlight`;
  let end = i;
  let start = end - content.length + 1;
  const last = content.length - 1;
  if (end >= lines.length || !lines[end][raw] == content[last]) {
    return;
  }
  if (start < 0) {
    content = content.slice(-start);
    start = 0;
    startOffset = 0;
    last = content.length - 1;
  }
  lines[start][dst] = [' ' + lines[start][raw].substring(1, startOffset + 1),
                       '*' + lines[start][raw].substring(startOffset + 1)];
  for (let i = start+1; i < end; i++) {
    lines[i][dst] = ['*' + lines[i][raw].substring(1)];
  }
  if (start == end) {
    lines[end][dst][1] = lines[end][dst][1].substring(0, endOffset + 1);
  } else {
    lines[end][dst] = ['*' + lines[end][raw].substring(1, endOffset + 1)];
  }
}
