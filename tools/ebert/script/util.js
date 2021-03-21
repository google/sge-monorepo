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

function Unix2Date(unixtime) {
  if (!unixtime || unixtime == 0) {
    return ""
  }
  var d = new Date(unixtime * 1000);
  return d.toDateString() + ' ' +
    ('0' + d.getHours()).slice(-2) + ':' +
    ('0' + d.getMinutes()).slice(-2);
}

function AvatarImg(user) {
  return 'https://picsum.photos/200'
}

function ReviewColor(review) {
  if (review.commits.length > 0) {
    return "#f5f5f5";
  } else if (review.state == "approved") {
    return "#f4fff7";
  } else {
    return "#fef7e0";
  }
}

function Linkify(rawDescription) {
  const regexs = [{
    // web links:
    // - start with whitespace (tab, space, newline), colon, semi,
    //   dash, open paren, open bracket
    // - followed by the literal "http://" or "https://"
    // - end with 1 or more non-whitespace, close-paren, close bracket
    re: /[\s:;(\[-](https?:\/\/[^\s)\]]+)/g,
    subfunc: (match) =>
      `<a target='_blank' href='${match}'>${match}</a>`,
  }, {
    // Bug links:
    // - start with whitespace (tab, space, newline), colon, semi,
    //   dash, open paren, open bracket
    // - followed by the literal "b/"
    // - end with 1 or more digits
    re: /[\s:;(\[-](b\/\d+)/g,
    subfunc: (match) =>
      `<a target='_blank' href='http://${match}'>${match}</a>`,
  }];

  let linkified = rawDescription;

  regexs.forEach((regex) => {
    let matches = linkified.matchAll(regex.re);
    let matched = {};
    for (const match of matches) {
      if (matched[match[1]]) {
        continue;
      }
      matched[match[1]] = true;
      linkified = linkified.replaceAll(match[1], regex.subfunc(match[1]));
    }
  })
  return linkified;
}
