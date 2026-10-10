const fs = require('fs');
const path = require('path');

const distDir = path.join(__dirname, 'dist');

function copyDir(src, dest) {
  if (!fs.existsSync(src)) return;
  fs.mkdirSync(dest, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);
    if (entry.isDirectory()) copyDir(srcPath, destPath);
    else fs.copyFileSync(srcPath, destPath);
  }
}

fs.rmSync(distDir, { recursive: true, force: true });
fs.mkdirSync(distDir, { recursive: true });
copyDir(path.join(__dirname, 'wailsjs'), path.join(distDir, 'wailsjs'));
fs.copyFileSync(path.join(__dirname, 'index.html'), path.join(distDir, 'index.html'));
copyDir(path.join(__dirname, 'css'), path.join(distDir, 'css'));
copyDir(path.join(__dirname, 'assets'), path.join(distDir, 'assets'));

const fontsDir = path.join(__dirname, 'fonts');
if (fs.existsSync(fontsDir)) copyDir(fontsDir, path.join(distDir, 'fonts'));

const jsDir = path.join(__dirname, 'js');
const distJsDir = path.join(distDir, 'js');
fs.mkdirSync(distJsDir, { recursive: true });
for (const file of fs.readdirSync(jsDir)) {
  if (file.endsWith('.js')) fs.copyFileSync(path.join(jsDir, file), path.join(distJsDir, file));
}

console.log('Build completed: frontend/dist/');
