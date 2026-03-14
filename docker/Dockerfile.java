# ==============================================================================
# HotPlex Java Development Stack (2026)
# ==============================================================================

# 1. Source the binary provider
FROM hotplex:artifacts AS binary-provider

# 2. SDK Layer
FROM bellsoft/liberica-openjdk-debian:25 AS sdk-source

# 3. Target Foundation
FROM hotplex:base

# Inject JDK (Cached)
USER root
COPY --from=sdk-source /usr/lib/jvm/jdk /opt/java/openjdk
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${JAVA_HOME}/bin:${PATH}"

# 4. Build & Diagnostic Tools (Cached)

# Gradle
RUN ENV_GRADLE_VERSION=8.14 && \
    wget -q https://services.gradle.org/distributions/gradle-${ENV_GRADLE_VERSION}-bin.zip && \
    unzip -q gradle-${ENV_GRADLE_VERSION}-bin.zip -d /opt && rm gradle-${ENV_GRADLE_VERSION}-bin.zip

# Maven
RUN ENV_MAVEN_VERSION=3.9.13 && \
    wget -q https://archive.apache.org/dist/maven/maven-3/${ENV_MAVEN_VERSION}/binaries/apache-maven-${ENV_MAVEN_VERSION}-bin.tar.gz && \
    tar -xzf apache-maven-${ENV_MAVEN_VERSION}-bin.tar.gz -C /opt && rm apache-maven-${ENV_MAVEN_VERSION}-bin.tar.gz

# Spring Boot CLI
RUN ENV_SPRING_VERSION=4.0.3 && \
    wget -q https://repo.maven.apache.org/maven2/org/springframework/boot/spring-boot-cli/${ENV_SPRING_VERSION}/spring-boot-cli-${ENV_SPRING_VERSION}-bin.tar.gz && \
    tar -xzf spring-boot-cli-${ENV_SPRING_VERSION}-bin.tar.gz -C /opt && rm spring-boot-cli-${ENV_SPRING_VERSION}-bin.tar.gz

ENV GRADLE_HOME=/opt/gradle-8.14
ENV MAVEN_HOME=/opt/apache-maven-3.9.13
ENV SPRING_HOME=/opt/spring-4.0.3
ENV PATH="${GRADLE_HOME}/bin:${MAVEN_HOME}/bin:${SPRING_HOME}/bin:${PATH}"

# ==============================================================================
# 🔥 Late Injection: The Binary (Changes frequently)
# ==============================================================================
COPY --from=binary-provider /hotplexd /usr/local/bin/hotplexd
# ==============================================================================

USER hotplex
CMD ["/usr/local/bin/hotplexd"]
LABEL org.opencontainers.image.title="HotPlex Java"
