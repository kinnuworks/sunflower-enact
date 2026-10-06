# Build
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /app
COPY pom.xml .
RUN mvn -q dependency:go-offline
COPY src ./src
RUN mvn -q clean package -DskipTests

# Run
FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=build /app/target/greencharge.jar app.jar
EXPOSE 8080
# No browser inside the container.
ENV APP_OPEN_BROWSER=false
ENTRYPOINT ["java", "-jar", "app.jar"]
