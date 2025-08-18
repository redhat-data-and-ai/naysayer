# 🤝 Contributing to Naysayer

Thank you for your interest in contributing to Naysayer!

## 🚀 Quick Start

1. **Fork and clone** the repository
2. **Create a branch**: `git checkout -b feature/your-feature-name`
3. **Make changes** following our quality standards
4. **Test your changes**: `make test`
5. **Submit a pull request**

> **📚 Detailed Setup**: See [Development Guide](DEVELOPMENT.md) for complete environment setup and project structure.

## 🎯 What You Can Contribute

- **🛡️ New validation rules** for different file types
- **🐛 Bug fixes** and performance improvements  
- **📚 Documentation** improvements and examples
- **🔧 Infrastructure** enhancements and CI/CD improvements

## 📝 Contribution Standards

### Commit Messages
Follow [Conventional Commits](https://conventionalcommits.org/):
```
feat(rules): add validation for new file type
fix(webhook): handle timeout errors gracefully
docs(api): update endpoint documentation
```

### Quality Requirements
- ✅ **Test Coverage**: Minimum 80% for new code
- ✅ **Documentation**: Update relevant docs in `docs/`
- ✅ **Security**: No hardcoded secrets, proper input validation
- ✅ **Code Review**: All changes must be reviewed

### Building Rules
New validation rules require:
1. **Implementation**: Implement the Rule interface with line-level validation methods:
   ```go
   type Rule interface {
       Name() string
       Description() string
       GetCoveredLines(filePath string, fileContent string) []LineRange
       ValidateLines(filePath string, fileContent string, lineRanges []LineRange) (DecisionType, string)
   }
   ```
2. **Testing**: Comprehensive tests for line-level validation
3. **Documentation**: Rule behavior guide in `docs/rules/`
4. **YAML Awareness**: Support for YAML section parsing if applicable

> **📖 Complete Guide**: See [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md) for detailed rule development process.

## 🔍 Code Review Process

All contributions go through code review focusing on:
- **Functionality**: Does it solve the intended problem?
- **Quality**: Is the code readable and maintainable? 
- **Testing**: Are edge cases and errors handled?
- **Documentation**: Are changes properly documented?

**Review Timeline**: Initial review within 2-3 business days.

## 🆘 Getting Help

- **Development**: [Development Guide](DEVELOPMENT.md)
- **Rule Creation**: [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md)  
- **Testing**: [Rule Testing Guide](docs/RULE_TESTING_GUIDE.md)
- **Deployment**: [Deployment Guide](DEPLOYMENT.md)

For questions, open an issue or start a discussion.

## 📄 License

By contributing, you agree that your contributions will be licensed under the same license as the project (Apache 2.0 and MIT dual license).

---

**🚀 Ready to contribute?** Start with [Development Guide](DEVELOPMENT.md) for setup, then explore [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md) for building validation rules.