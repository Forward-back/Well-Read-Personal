#!/usr/bin/env python3
"""
Gaze MCP Server 安装脚本
"""

from setuptools import setup


# 读取 README 文件
def read_readme():
    try:
        with open("README.md", "r", encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return "Gaze MCP Server - Model Context Protocol server for Gaze API"


# 读取依赖
def read_requirements():
    try:
        with open("requirements.txt", "r", encoding="utf-8") as f:
            return [
                line.strip() for line in f if line.strip() and not line.startswith("#")
            ]
    except FileNotFoundError:
        return [
            "mcp>=2,<3",
            "requests>=2.31.0",
            "starlette>=0.27.0",
            "uvicorn>=0.24.0",
        ]


setup(
    name="ylsw-wellread-mcp",
    version="1.2.0",
    author="Gaze Team",
    author_email="support@gaze.com",
    description="Gaze MCP Server - Model Context Protocol server for Gaze API",
    long_description=read_readme(),
    long_description_content_type="text/markdown",
    url="https://wellread/tree/main/mcp",
    py_modules=["gaze_mcp_server", "client", "tools", "main", "run_server", "run"],
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Internet :: WWW/HTTP :: HTTP Servers",
        "Topic :: Scientific/Engineering :: Artificial Intelligence",
    ],
    python_requires=">=3.10",
    install_requires=read_requirements(),
    entry_points={
        "console_scripts": [
            "ylsw-wellread-mcp=main:sync_main",
            "wellread-server=run_server:main",
        ],
    },
    include_package_data=True,
    data_files=[
        ("", ["README.md", "requirements.txt", "LICENSE"]),
    ],
    keywords="mcp model-context-protocol gaze knowledge-management api-server",
)
