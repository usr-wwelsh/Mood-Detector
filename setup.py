from setuptools import setup, find_packages

setup(
    name="mood-detector",
    version="0.1.0",
    author="usr-wwelsh",
    description="Open-source music mood analysis",
    long_description=open("README.md", encoding="utf-8").read(),
    long_description_content_type="text/markdown",
    url="https://github.com/usr-wwelsh/mood-detector",
    packages=find_packages(),
    install_requires=[
        "librosa>=0.11.0",  # 0.11.0+ required for scipy 1.15+ compatibility
        "numpy>=1.24.0",
    ],
    extras_require={
        "api": [
            "fastapi>=0.104.1",
            "uvicorn>=0.24.0",
            "python-multipart>=0.0.6"
        ]
    },
    entry_points={
        'console_scripts': [
            'mood=cli.mood:main',  # Creates "mood" command
        ],
    },
    python_requires=">=3.8",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
)