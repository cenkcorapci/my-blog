function renderBenchmarkChart(headingId) {
    const heading = document.getElementById(headingId);
    const table = heading && heading.nextElementSibling;
    if (!table || table.tagName !== 'TABLE') return;

    const headers = Array.from(table.querySelectorAll('thead th')).slice(1);
    const rows = Array.from(table.querySelectorAll('tbody tr')).map(row =>
        Array.from(row.querySelectorAll('td')).map(cell => cell.textContent.trim())
    );
    const values = rows.flatMap(row => row.slice(1).map(Number));
    const max = Math.max(...values);
    if (!rows.length || !headers.length || !Number.isFinite(max) || max <= 0 ||
        rows.some(row => row.length !== headers.length + 1) ||
        values.some(value => !Number.isFinite(value) || value < 0)) return;

    const chart = document.createElement('div');
    chart.className = 'benchmark-chart';
    chart.setAttribute('aria-hidden', 'true');
    rows.forEach(row => {
        const group = document.createElement('div');
        group.className = 'benchmark-chart-group';
        const label = document.createElement('div');
        label.className = 'benchmark-chart-label';
        label.textContent = `${row[0]} vectors`;
        group.appendChild(label);

        row.slice(1).forEach((value, i) => {
            const line = document.createElement('div');
            line.className = 'benchmark-chart-row';
            const name = document.createElement('span');
            name.textContent = headers[i].textContent;
            const track = document.createElement('div');
            track.className = 'benchmark-chart-track';
            const bar = document.createElement('div');
            bar.className = 'benchmark-chart-bar';
            bar.style.width = `${Number(value) / max * 80}%`;
            const amount = document.createElement('span');
            amount.className = 'benchmark-chart-value';
            amount.textContent = value;
            track.append(bar, amount);
            line.append(name, track);
            group.appendChild(line);
        });
        chart.appendChild(group);
    });
    table.before(chart);
}

if (typeof document !== 'undefined') {
    [
        'peak-process-rss-by-index-size',
        'exact-search-latency-by-index-size',
        'exact-search-throughput-by-index-size'
    ].forEach(renderBenchmarkChart);
}

if (typeof module !== 'undefined') module.exports = { renderBenchmarkChart };
