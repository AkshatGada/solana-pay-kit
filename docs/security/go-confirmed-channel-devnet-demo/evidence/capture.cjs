const { chromium } = require('/Users/akshatgada/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const path = require('node:path');
(async () => {
  const browser = await chromium.launch({headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
  const page = await browser.newPage({viewport: {width: 1440, height: 1100}, deviceScaleFactor: 1});
  await page.route('http://**/*', route => route.abort());
  await page.route('https://**/*', route => route.abort());
  await page.goto('file://' + path.join(__dirname, 'report.html'));
  await page.locator('#summary').screenshot({path: path.join(__dirname, 'summary.png')});
  await page.locator('#http-proof').screenshot({path: path.join(__dirname, 'http-proof.png')});
  console.log('Saved real browser captures of the rendered evidence report.');
  await browser.close();
})().catch(err => {console.error(err);process.exit(1)});
