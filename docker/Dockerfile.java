# ============================================
# HotPlex Java/Kotlin Stack
# ============================================

ARG BASE_IMAGE=hotplex:base
FROM ${BASE_IMAGE}

USER root
ARG ALPINE_MIRROR=dl-cdn.alpinelinux.org

# ============================================
# Java/Kotlin Stack Extensions
# ============================================
RUN if [ "$ALPINE_MIRROR" != "dl-cdn.alpinelinux.org" ]; then \
        sed -i "s/dl-cdn.alpinelinux.org/$ALPINE_MIRROR/g" /etc/apk/repositories; \
    fi && \
    apk add --no-cache \
    openjdk25 \
    fontconfig \
    ttf-dejavu

# Gradle
ENV GRADLE_VERSION=8.14
RUN wget -q https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip && \
    unzip gradle-${GRADLE_VERSION}-bin.zip && \
    mv gradle-${GRADLE_VERSION} /opt/gradle && \
    rm gradle-${GRADLE_VERSION}-bin.zip
ENV GRADLE_HOME=/opt/gradle
ENV PATH="${GRADLE_HOME}/bin:${PATH}"

# Maven
ENV MAVEN_VERSION=3.9.13
RUN wget -q https://archive.apache.org/dist/maven/maven-3/${MAVEN_VERSION}/binaries/apache-maven-${MAVEN_VERSION}-bin.tar.gz && \
    tar -xzf apache-maven-${MAVEN_VERSION}-bin.tar.gz && \
    mv apache-maven-${MAVEN_VERSION} /opt/maven && \
    rm apache-maven-${MAVEN_VERSION}-bin.tar.gz
ENV MAVEN_HOME=/opt/maven
ENV PATH="${MAVEN_HOME}/bin:${PATH}"

# Kotlin
ENV KOTLIN_VERSION=2.3.0
RUN wget -q https://github.com/JetBrains/kotlin/releases/download/v${KOTLIN_VERSION}/kotlin-compiler-${KOTLIN_VERSION}.zip && \
    unzip kotlin-compiler-${KOTLIN_VERSION}.zip && \
    mv kotlinc /opt/kotlinc && \
    rm kotlin-compiler-${KOTLIN_VERSION}.zip
ENV KOTLIN_HOME=/opt/kotlinc
ENV PATH="${KOTLIN_HOME}/bin:${PATH}"

# Linter Tools
RUN wget -qO /usr/local/bin/ktlint "https://github.com/pinterest/ktlint/releases/download/1.5.0/ktlint" && \
    chmod +x /usr/local/bin/ktlint && \
    mkdir -p /opt/detekt && \
    wget -qO /opt/detekt/detekt-cli.jar "https://github.com/detekt/detekt/releases/download/v1.23.7/detekt-cli-1.23.7.jar" && \
    printf '#!/bin/bash\njava -jar /opt/detekt/detekt-cli.jar "$@"' > /usr/local/bin/detekt && \
    chmod +x /usr/local/bin/detekt

# Verification
RUN java --version && \
    gradle --version && \
    mvn --version && \
    kotlinc -version

# User setup
RUN mkdir -p /home/hotplex/.gradle /home/hotplex/.m2 && \
    chown -R hotplex:hotplex /home/hotplex

ENV GRADLE_USER_HOME=/home/hotplex/.gradle
ENV MAVEN_OPTS="-Xmx512m"

LABEL org.opencontainers.image.title="HotPlex Java"
LABEL org.opencontainers.image.description="HotPlex AI Agent with Go + Java/Kotlin stack"

USER hotplex
