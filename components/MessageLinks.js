.pragma library

// One boundary for detection AND activation. No URL decoding or normalization:
// the browser receives exactly the displayed URL, only after an explicit action.
function valid(value) {
    if (typeof value !== 'string' || value.length > 10000
            || /[\s<>"'\\\x00-\x1f\x7f-\x9f\u200b-\u200f\u202a-\u202e\u2060-\u206f\ufeff]/.test(value)
            || /%(?![0-9a-f]{2})|%(?:0[0-9a-f]|1[0-9a-f]|7f)/i.test(value)) return false
    var parts = /^https?:\/\/([^/?#]+)(?:[/?#].*)?$/i.exec(value)
    if (!parts || parts[1].indexOf('@') !== -1) return false
    var authority = parts[1], host, port = ''
    if (authority.charAt(0) === '[') {
        var ipv6 = /^\[([0-9a-f:]+)\](?::([0-9]+))?$/i.exec(authority)
        if (!ipv6) return false
        host = ipv6[1]; port = ipv6[2] || ''
        var groups = host.split(':'), compressed = host.indexOf('::') !== -1
        if (host.indexOf(':::') !== -1 || host.indexOf('::') !== host.lastIndexOf('::')
                || (host.startsWith(':') && !host.startsWith('::'))
                || (host.endsWith(':') && !host.endsWith('::'))
                || groups.some(g => g.length > 4)
                || (compressed ? groups.filter(g => g !== '').length >= 8 : groups.length !== 8 || groups.some(g => g === ''))) return false
    } else {
        var dns = /^([^:]+)(?::([0-9]+))?$/.exec(authority)
        if (!dns) return false
        host = dns[1]; port = dns[2] || ''
        if (host.length > 253) return false
        // ASCII DNS (including IDNA/punycode), IPv4, and local hostnames.
        // Unicode paths are preserved; ambiguous Unicode authorities stay text.
        if (!host.split('.').every(label => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label))) return false
        if (/^[0-9.]+$/.test(host) && (host.split('.').length !== 4 || host.split('.').some(n => Number(n) > 255 || (n.length > 1 && n.charAt(0) === '0')))) return false
        if (/^(?:0x[0-9a-f]+|[0-9]+)(?:\.(?:0x[0-9a-f]+|[0-9]+))*$/i.test(host) && /x/i.test(host)) return false
    }
    return !port || (Number(port) > 0 && Number(port) <= 65535)
}

function trimEnd(value) {
    var pairs = {')':'(',']':'[','}':'{'}, counts = {'(':0,')':0,'[':0,']':0,'{':0,'}':0}
    for (var i=0; i<value.length; i++) {
        var character=value.charAt(i)
        if (Object.prototype.hasOwnProperty.call(counts,character)) counts[character]++
    }
    var end=value.length
    while (end>0) {
        var last=value.charAt(end-1)
        if (/[.,!?;:]/.test(last)) end--
        else if (Object.prototype.hasOwnProperty.call(pairs,last) && counts[last]>counts[pairs[last]]) { counts[last]--;end-- }
        else break
    }
    return value.slice(0,end)
}

function spans(text) {
    if (typeof text !== 'string' || text.length > 10000) return []
    var result = [], match
    // Japanese prose delimiters are not swallowed into a neighboring URL.
    var pattern = /https?:\/\/[^\s<>"'「」『』【】〈〉《》（）｛｝［］、。！？]+/gi
    while ((match = pattern.exec(text)) !== null) {
        if (match.index > 0 && /[a-z0-9_:/@]/i.test(text.charAt(match.index - 1))) continue
        var url = trimEnd(match[0])
        if (valid(url)) result.push({start:match.index,end:match.index+url.length,url:url})
    }
    return result
}

function urls(text) { return spans(text).map(span => span.url) }

function escape(text) {
    return text.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
        .replace(/"/g,'&quot;').replace(/'/g,'&#39;')
}

// SECURITY: this is the only source for message RichText. Every raw substring
// and href is escaped before inserting fixed, application-authored markup.
// Never pass raw body, HTML, CSS, images, or a remote resource to the renderer.
function markup(text, links, color) {
    var output = '', position = 0
    var safeColor = /^#[0-9a-f]{6}$/i.test(color) ? color : '#83b487'
    for (var link of links) {
        output += escape(text.slice(position, link.start))
        output += '<a href="'+escape(link.url)+'" style="color:'+safeColor+';text-decoration:underline">'+escape(text.slice(link.start,link.end))+'</a>'
        position = link.end
    }
    output += escape(text.slice(position))
    // pre-wrap preserves spaces/tabs and actual newlines, including empty lines.
    return '<span style="white-space:pre-wrap">'+output+'</span>'
}
