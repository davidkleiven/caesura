# 🎼 Caesura

<div align="center">

![Caesura Logo](https://img.shields.io/badge/Caesura-Music%20Management-blue?style=for-the-badge&logo=music&logoColor=white)

**The modern solution for orchestras, bands, and choirs to store, manage, and distribute music**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go&logoColor=white)](https://golang.org/)
[![License](https://img.shields.io/badge/License-BSL%201.1-green?style=flat)](LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/davidkleiven/caesura/go.yml?style=flat&logo=github)](https://github.com/davidkleiven/caesura/actions)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-brightgreen?style=flat)](https://github.com/davidkleiven/caesura)

[![Features](#-features)](#-features)
[![Quick Start](#-quick-start)](#-quick-start)
[![Documentation](#-documentation)](#-documentation)
[![Contributing](#-contributing)](#-contributing)

</div>

---

## 🌟 Features

### 🎵 **Music Management**

- 📁 **Score Storage** - Upload and organize musical scores
- 🔍 **Smart Search** - Find instruments and compositions quickly
- 📄 **PDF Processing** - Built-in PDF handling and viewing
- 🎼 **Instrument Catalog** - Comprehensive instrument database

### 👥 **Organization & Collaboration**

- 🏢 **Multi-Organization Support** - Manage multiple groups
- 👤 **User Management** - Role-based access control
- 🔐 **Secure Authentication** - OAuth2 integration with Google
- 📧 **Email Notifications** - Automated communication system

### 💳 **Billing & Subscriptions**

- 💰 **Stripe Integration** - Professional payment processing
- 📊 **Subscription Management** - Flexible pricing tiers
- 🆓 **Free Tier** - Get started with up to 10 scores
- 💎 **Premium Plans** - Monthly and annual billing options

### 🛠️ **Technical Excellence**

- ☁️ **Cloud Native** - Google Cloud Platform integration
- 🔥 **Firestore Database** - Scalable NoSQL storage
- 📦 **Blob Storage** - Efficient file management
- 🎨 **Modern UI** - Responsive web interface with Tailwind CSS

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.24+** - [Install Go](https://golang.org/dl/)
- **Node.js & npm** - For CSS compilation

### Installation

```bash
# Clone the repository
git clone https://github.com/davidkleiven/caesura.git
cd caesura

# Install dependencies
go mod download

# Build the application
make build

# Compile CSS styles
make css
```

### Configuration

```bash
# Copy configuration template
cp pkg/profiles/config-local.yml config.yml

# Edit your configuration
# Set up Google Cloud credentials, Stripe keys, etc.
```

### Running the Application

```bash
# Development mode
make run

# Or run directly
./caesura
```

Visit `http://localhost:8080` to access Caesura! 🎉

---

## 📖 Documentation

### Configuration Profiles

- `config-local.yml` - Local development
- `config-test.yml` - Testing environment
- `config-ci.yml` - Continuous integration
- `config-large-demo.yml` - Demo with extensive data

### Database Schema

Caesura uses Google Firestore with the following main collections:

- **Users** - User accounts and profiles
- **Organizations** - Musical groups and ensembles
- **Scores** - Musical compositions and metadata
- **Subscriptions** - Billing and plan information

---

## 🧪 Testing

```bash
# Run all tests
make test

# Unit tests only
make unittest

# UI/E2E tests
make uitest

# Generate coverage report
make unittest
# Open coverage.html in your browser
```

---

## 🛠️ Development

### Project Structure

```
caesura/
├── api/           # HTTP handlers and routing
├── pkg/           # Core business logic
├── web/           # Frontend assets and templates
├── cmd/           # Command-line utilities
├── testutils/     # Testing helpers
└── web_test/      # End-to-end tests
```

### Code Quality

- ✅ **Pre-commit hooks** - Automatic code formatting
- ✅ **CI/CD Pipeline** - Automated testing and deployment
- ✅ **Code Coverage** - Target: 85%+
- ✅ **Type Safety** - Strong typing with Go

---

## 🌐 Deployment

### Docker

```bash
# Build the image
docker build -t caesura .

# Run the container
docker run -p 8080:8080 caesura
```

## 🤝 Contributing

We welcome contributions! 🎉

### How to Contribute

1. **Report Issues** - Found a bug? [Create an issue](https://github.com/davidkleiven/caesura/issues)
2. **Feature Requests** - Have an idea? [Open a discussion](https://github.com/davidkleiven/caesura/discussions)
3. **Pull Requests** - Ready to code? [Submit a PR](https://github.com/davidkleiven/caesura/pulls)

### Development Guidelines

- Follow Go best practices and idioms
- Write comprehensive tests
- Update documentation for new features
- Use conventional commit messages

---

## 📄 License

This project is licensed under the **Business Source License 1.1** - see the [LICENSE](LICENSE) file for details.

**Change Date:** Four years after public release
**Change License:** Mozilla Public License 2.0

---

## 🙏 Acknowledgments

- **Google Cloud Platform** - Cloud infrastructure and services
- **Stripe** - Payment processing
- **Gorilla Toolkit** - Web framework components
- **Tailwind CSS** - Styling and UI framework

---

<div align="center">

**Made with ❤️ for musicians everywhere**

[![Back to top](https://img.shields.io/badge/Back%20to%20Top-↑-blue?style=flat)](#-caesura)

</div>
