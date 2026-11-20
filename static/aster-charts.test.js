/** @jest-environment jsdom */
const { renderBenchmarkChart, renderBenchmarkComparison } = require('./aster-charts');

test('renders log-scale series and points from the accessible data table', () => {
    document.body.innerHTML = `
        <h4 id="test-chart">Benchmark</h4>
        <table>
            <thead><tr><th>Vectors</th><th>p50 (ms)</th><th>p95 (ms)</th></tr></thead>
            <tbody><tr><td>10,000</td><td>5</td><td>10</td></tr>
                <tr><td>100,000</td><td>10</td><td>20</td></tr>
                <tr><td>1,000,000</td><td>50</td><td>100</td></tr></tbody>
        </table>`;
    renderBenchmarkChart('test-chart');
    const chart = document.querySelector('.benchmark-chart');
    expect(chart.getAttribute('aria-hidden')).toBe('true');
    expect(chart.textContent).toContain('1,000,000 vectors');
    expect(chart.textContent).toContain('p95 (ms)');
    expect(chart.querySelectorAll('polyline')).toHaveLength(2);
    expect(chart.querySelectorAll('circle')).toHaveLength(6);
    expect(chart.querySelectorAll('circle')[0].getAttribute('cx')).toBe('75');
    expect(chart.querySelectorAll('circle')[1].getAttribute('cx')).toBe('387.5');
    expect(chart.querySelectorAll('circle')[2].getAttribute('cx')).toBe('700');
    expect(document.querySelector('table')).not.toBeNull();
});

test('leaves invalid or missing benchmark tables alone', () => {
    document.body.innerHTML = '<h4 id="test-chart">Benchmark</h4><table><thead><tr><th>Size</th><th>Value</th></tr></thead><tbody><tr><td>10</td><td>unknown</td></tr></tbody></table>';
    renderBenchmarkChart('missing');
    renderBenchmarkChart('test-chart');
    expect(document.querySelector('.benchmark-chart')).toBeNull();
    expect(document.querySelector('table')).not.toBeNull();
});

test('renders independently scaled comparison bars while retaining the table', () => {
    document.body.innerHTML = `
        <h4 id="comparison">Comparison</h4><p>Conditions</p>
        <table><thead><tr><th>Metric</th><th>1.0.1</th><th>1.0.2</th></tr></thead>
        <tbody><tr><td>Latency</td><td>100</td><td>25</td></tr>
            <tr><td>Throughput</td><td>2</td><td>8</td></tr></tbody></table>`;
    renderBenchmarkComparison('comparison');
    const chart = document.querySelector('.benchmark-chart');
    expect(chart.getAttribute('aria-hidden')).toBe('true');
    expect(chart.textContent).toContain('1.0.2');
    expect(Array.from(chart.querySelectorAll('rect')).map(bar => bar.getAttribute('width')))
        .toEqual(['440', '110', '110', '440']);
    expect(document.querySelector('table')).not.toBeNull();
});

test('does not render a comparison with invalid values', () => {
    document.body.innerHTML = `<h4 id="comparison">Comparison</h4>
        <table><thead><tr><th>Metric</th><th>Old</th><th>New</th></tr></thead>
        <tbody><tr><td>Latency</td><td>unknown</td><td>1</td></tr></tbody></table>`;
    renderBenchmarkComparison('comparison');
    expect(document.querySelector('.benchmark-chart')).toBeNull();
    expect(document.querySelector('table')).not.toBeNull();
});

test('automatically renders the 1.0.3 comparison', () => {
    document.body.innerHTML = `<h4 id="aster-103-another-10-million-vector-improvement">1.0.3</h4>
        <p>Conditions</p><table>
        <thead><tr><th>Metric</th><th>1.0.2</th><th>1.0.3</th></tr></thead>
        <tbody><tr><td>Latency</td><td>252.060</td><td>214.753</td></tr></tbody></table>`;
    jest.isolateModules(() => require('./aster-charts'));
    expect(document.querySelectorAll('.benchmark-comparison-bar')).toHaveLength(2);
    expect(document.querySelector('table')).not.toBeNull();
});
