const expo = require('eslint-config-expo/flat')
module.exports = [
  ...expo,
  { files: ['jest.setup.js', '__tests__/**'], languageOptions: { globals: { jest: 'readonly' } } },
  { ignores: ['node_modules', '.expo'] },
]
