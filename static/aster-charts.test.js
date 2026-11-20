/** @jest-environment jsdom */
const { renderBenchmarkChart } = require('./aster-charts');

test('renders chart bars from the accessible data table', () => {
    document.body.innerHTML = `
        <h4 id="test-chart">Benchmark</h4>
        <table>
            <thead><tr><th>Vectors</th><th>p50 (ms)</th><th>p95 (ms)</th></tr></thead>
            <tbody><tr><td>10,000</td><td>5</td><td>10</td></tr></tbody>
        </table>`;
    renderBenchmarkChart('test-chart');
    const chart = document.querySelector('.benchmark-chart');
    expect(chart.getAttribute('aria-hidden')).toBe('true');
    expect(chart.textContent).toContain('10,000 vectors');
    expect(chart.textContent).toContain('p95 (ms)');
    expect(chart.querySelectorAll('.benchmark-chart-bar')).toHaveLength(2);
    expect(chart.querySelector('.benchmark-chart-bar').style.width).toBe('40%');
    expect(document.querySelector('table')).not.toBeNull();
});

test('leaves invalid or missing benchmark tables alone', () => {
    document.body.innerHTML = '<h4 id="test-chart">Benchmark</h4><table><thead><tr><th>Size</th><th>Value</th></tr></thead><tbody><tr><td>10</td><td>unknown</td></tr></tbody></table>';
    renderBenchmarkChart('missing');
    renderBenchmarkChart('test-chart');
    expect(document.querySelector('.benchmark-chart')).toBeNull();
    expect(document.querySelector('table')).not.toBeNull();
});
