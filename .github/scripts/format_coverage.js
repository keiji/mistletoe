module.exports = function formatCoverageReport(output) {
    const lines = output.trim().split('\n');
    let totalCoverage = 'Unknown';
    let tableRows = [];

    tableRows.push('| File | Function | Coverage |');
    tableRows.push('| :--- | :--- | ---: |');

    for (const line of lines) {
        const parts = line.split(/\s+/);
        if (parts.length < 3) continue;

        if (parts[0] === 'total:') {
            totalCoverage = parts[parts.length - 1];
        } else {
            tableRows.push(`| ${parts[0]} | ${parts[1]} | ${parts[2]} |`);
        }
    }

    let body = `## Test Coverage Report\n`;
    body += `**Total Coverage:** ${totalCoverage}\n\n`;
    body += `<details>\n<summary>Detailed Report</summary>\n\n`;
    body += tableRows.join('\n');
    body += `\n</details>`;

    return body;
};
