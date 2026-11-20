function renderBenchmarkChart(headingId) {
    const heading = document.getElementById(headingId);
    const table = heading && heading.nextElementSibling;
    if (!table || table.tagName !== 'TABLE') return;

    const headers = Array.from(table.querySelectorAll('thead th')).slice(1);
    const rows = Array.from(table.querySelectorAll('tbody tr')).map(row =>
        Array.from(row.querySelectorAll('td')).map(cell => cell.textContent.trim())
    );
    const sizes = rows.map(row => Number(row[0].replace(/,/g, '')));
    const values = rows.flatMap(row => row.slice(1).map(Number));
    const max = Math.max(...values);
    if (rows.length < 2 || !headers.length || !Number.isFinite(max) || max <= 0 ||
        rows.some(row => row.length !== headers.length + 1) ||
        sizes.some((size, i) => !Number.isFinite(size) || size <= 0 ||
            (i > 0 && size <= sizes[i - 1])) ||
        values.some(value => !Number.isFinite(value) || value < 0)) return;

    const ns = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(ns, 'svg');
    svg.setAttribute('viewBox', '0 0 720 310');
    svg.setAttribute('aria-hidden', 'true');
    svg.classList.add('benchmark-chart');
    const add = (tag, attrs, text) => {
        const element = document.createElementNS(ns, tag);
        Object.entries(attrs).forEach(([key, value]) => element.setAttribute(key, value));
        if (text !== undefined) element.textContent = text;
        svg.appendChild(element);
        return element;
    };
    const left = 75, right = 700, top = 40, bottom = 245;
    const minLog = Math.log10(sizes[0]), maxLog = Math.log10(sizes[sizes.length - 1]);
    const x = size => left + (Math.log10(size) - minLog) / (maxLog - minLog) * (right - left);
    const y = value => bottom - value / max * (bottom - top);

    [0, 0.5, 1].forEach(fraction => {
        const level = y(max * fraction);
        add('line', { x1: left, y1: level, x2: right, y2: level, class: 'benchmark-grid' });
        add('text', { x: left - 10, y: level + 4, 'text-anchor': 'end' },
            Number((max * fraction).toPrecision(3)).toLocaleString('en-US'));
    });
    for (let power = Math.ceil(minLog); power <= Math.floor(maxLog); power++) {
        const size = 10 ** power;
        const label = size >= 1000000 ? `${size / 1000000}m` : `${size / 1000}k`;
        add('text', { x: x(size), y: bottom + 20, 'text-anchor': 'middle' }, label);
    }
    add('text', { x: (left + right) / 2, y: 295, 'text-anchor': 'middle' },
        'Vectors (log scale)');
    headers.forEach((header, i) => {
        const points = rows.map((row, j) => `${x(sizes[j])},${y(Number(row[i + 1]))}`).join(' ');
        add('polyline', { points, class: `benchmark-series benchmark-series-${i}` });
        rows.forEach((row, j) => {
            const circle = add('circle', {
                cx: x(sizes[j]), cy: y(Number(row[i + 1])), r: 4,
                class: `benchmark-point benchmark-series-${i}`
            });
            const title = document.createElementNS(ns, 'title');
            title.textContent = `${row[0]} vectors: ${row[i + 1]} ${header.textContent}`;
            circle.appendChild(title);
        });
        add('text', { x: left + i * 180, y: 20, class: `benchmark-legend benchmark-series-${i}` },
            header.textContent);
    });
    table.before(svg);
}

function renderBenchmarkComparison(headingId) {
    const heading = document.getElementById(headingId);
    let table = heading && heading.nextElementSibling;
    while (table && table.tagName !== 'TABLE' && !/^H[1-6]$/.test(table.tagName)) {
        table = table.nextElementSibling;
    }
    if (!table || table.tagName !== 'TABLE') return;

    const versions = Array.from(table.querySelectorAll('thead th')).slice(1)
        .map(cell => cell.textContent.trim());
    const rows = Array.from(table.querySelectorAll('tbody tr')).map(row =>
        Array.from(row.querySelectorAll('td')).map(cell => cell.textContent.trim())
    );
    if (versions.length !== 2 || !rows.length || rows.some(row =>
        row.length !== 3 || row.slice(1).some(value => !Number.isFinite(Number(value)) ||
            Number(value) < 0) || Math.max(...row.slice(1).map(Number)) <= 0)) return;

    const ns = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(ns, 'svg');
    svg.setAttribute('viewBox', `0 0 720 ${rows.length * 94}`);
    svg.setAttribute('aria-hidden', 'true');
    svg.classList.add('benchmark-chart');
    const add = (tag, attrs, text) => {
        const element = document.createElementNS(ns, tag);
        Object.entries(attrs).forEach(([key, value]) => element.setAttribute(key, value));
        if (text !== undefined) element.textContent = text;
        svg.appendChild(element);
    };
    rows.forEach((row, i) => {
        const top = i * 94;
        const max = Math.max(...row.slice(1).map(Number));
        add('text', { x: 0, y: top + 16 }, row[0]);
        row.slice(1).forEach((value, j) => {
            const y = top + 26 + j * 29;
            const width = Number(value) / max * 440;
            add('text', { x: 0, y: y + 15 }, versions[j]);
            add('rect', { x: 90, y, width, height: 18,
                class: `benchmark-comparison-bar benchmark-series-${j}` });
            add('text', { x: 100 + width, y: y + 15 }, value);
        });
    });
    table.before(svg);
}

if (typeof document !== 'undefined') {
    [
        'peak-process-rss-by-index-size',
        'exact-search-latency-by-index-size',
        'exact-search-throughput-by-index-size'
    ].forEach(renderBenchmarkChart);
    renderBenchmarkComparison('aster-102-10-million-vector-comparison');
}

if (typeof module !== 'undefined') module.exports = { renderBenchmarkChart, renderBenchmarkComparison };
