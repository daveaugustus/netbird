package policies

import (
	"net/http"
	"regexp"

	"github.com/gorilla/mux"

	"github.com/netbirdio/netbird/management/server/account"
	nbcontext "github.com/netbirdio/netbird/management/server/context"
	"github.com/netbirdio/netbird/management/server/geolocation"
	"github.com/netbirdio/netbird/management/server/http/api"
	"github.com/netbirdio/netbird/management/server/http/util"
	"github.com/netbirdio/netbird/management/server/status"
)

var (
	countryCodeRegex = regexp.MustCompile("^[a-zA-Z]{2}$")
)

// geolocationsHandler is a handler that returns locations.
type geolocationsHandler struct {
	accountManager     account.Manager
	geolocationManager geolocation.Geolocation
}

func addLocationsEndpoint(accountManager account.Manager, locationManager geolocation.Geolocation, router *mux.Router) {
	locationHandler := newGeolocationsHandlerHandler(accountManager, locationManager)
	router.HandleFunc("/locations/countries", locationHandler.getAllCountries).Methods("GET", "OPTIONS")
	router.HandleFunc("/locations/countries/{country}/cities", locationHandler.getCitiesByCountry).Methods("GET", "OPTIONS")
}

// newGeolocationsHandlerHandler creates a new Geolocations handler
func newGeolocationsHandlerHandler(accountManager account.Manager, geolocationManager geolocation.Geolocation) *geolocationsHandler {
	return &geolocationsHandler{
		accountManager:     accountManager,
		geolocationManager: geolocationManager,
	}
}

// getAllCountries retrieves a list of all countries
func (l *geolocationsHandler) getAllCountries(w http.ResponseWriter, r *http.Request) {
	if err := l.authenticateUser(r); err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	if l.geolocationManager == nil {
		// TODO: update error message to include geo db self hosted doc link when ready
		util.WriteError(r.Context(), status.Errorf(status.PreconditionFailed, "Geo location database is not initialized"), w)
		return
	}

	allCountries, err := l.geolocationManager.GetAllCountries()
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	countries := make([]api.Country, 0, len(allCountries))
	for _, country := range allCountries {
		countries = append(countries, toCountryResponse(country))
	}
	util.WriteJSONObject(r.Context(), w, countries)
}

// getCitiesByCountry retrieves a list of cities based on the given country code
func (l *geolocationsHandler) getCitiesByCountry(w http.ResponseWriter, r *http.Request) {
	if err := l.authenticateUser(r); err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	vars := mux.Vars(r)
	countryCode := vars["country"]
	if !countryCodeRegex.MatchString(countryCode) {
		util.WriteError(r.Context(), status.Errorf(status.InvalidArgument, "invalid country code"), w)
		return
	}

	if l.geolocationManager == nil {
		util.WriteError(r.Context(), status.Errorf(status.PreconditionFailed, "Geo location database is not initialized. "+
			"Check the self-hosted Geo database documentation at https://docs.netbird.io/selfhosted/geo-support"), w)
		return
	}

	allCities, err := l.geolocationManager.GetCitiesByCountry(countryCode)
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	cities := make([]api.City, 0, len(allCities))
	for _, city := range allCities {
		cities = append(cities, toCityResponse(city))
	}
	util.WriteJSONObject(r.Context(), w, cities)
}

func (l *geolocationsHandler) authenticateUser(r *http.Request) error {
	userAuth, err := nbcontext.GetUserAuthFromContext(r.Context())
	if err != nil {
		return err
	}

	_, userID := userAuth.AccountId, userAuth.UserId

	user, err := l.accountManager.GetUserByID(r.Context(), userID)
	if err != nil {
		return err
	}

	if !user.HasAdminPower() {
		return status.Errorf(status.PermissionDenied, "user is not allowed to perform this action")
	}
	return nil
}

func toCountryResponse(country geolocation.Country) api.Country {
	return api.Country{
		CountryName: country.CountryName,
		CountryCode: country.CountryISOCode,
	}
}

func toCityResponse(city geolocation.City) api.City {
	return api.City{
		CityName:  city.CityName,
		GeonameId: city.GeoNameID,
	}
}
