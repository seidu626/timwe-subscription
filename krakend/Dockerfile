# Use the official KrakenD base image (official Docker library image)
FROM docker.io/library/krakend:latest

USER root

# Set the working directory for KrakenD
WORKDIR /etc/krakend

# Copy the local configuration files into the Docker image
COPY ./ /etc/krakend/

# Change permissions to allow writing to the configuration directory
RUN chmod -R 777 /etc/krakend && chmod -R 777 /tmp

# Set environment variables for KrakenD templates
ENV FC_ENABLE=1
ENV FC_SETTINGS="/etc/krakend/config/settings"
ENV FC_PARTIALS="/etc/krakend/config/partials"
ENV FC_TEMPLATES="/etc/krakend/config/templates"
ENV FC_OUT="/tmp/krakend.json"

# Expose the default KrakenD port
EXPOSE 8080

# Run as root to avoid permission issues
USER root

# Run KrakenD with the configuration
CMD ["krakend", "run", "-dc", "/etc/krakend/config/krakend.tmpl"]
