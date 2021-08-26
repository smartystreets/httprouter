package httprouter

import (
	"net/http"
	"strings"
	"unicode"
)

type routeResolver interface {
	// Resolve returns an instance of http.Handler and with a flag indicating if the route was understood.
	// If the http.Handler instance is not nil, the route was fully resolved and can be invoked.
	// If the http.Handler instance is nil AND the flag is true, the route was found, but the method isn't compatible (e.g. "POST /", but only a "GET /" was found.
	// If the http.Handler instance is nil AND the flag is false, the route was not found.
	Resolve(method Method, path string) (http.Handler, bool)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
type treeNode struct {
	pathFragment   string
	staticChildren []*treeNode
	variableChild  *treeNode
	wildcardChild  *treeNode
	handlers       map[Method]http.Handler
}

func main() {
	tree := &treeNode{}
	// tree.Add(routes)

	method := MethodGet
	incomingPath := "/path/whatever"
	_, resourceExists := tree.Resolve(method, incomingPath) // FIXME

	if resourceExists { //FIXME

	}
}

func (this *treeNode) Add(route Route) error {
	if len(route.Path) == 0 {
		if this.handlers[route.AllowedMethod] != nil { // The handler method already exists
			return ErrMethodAlreadyExists
		}
		this.handlers[route.AllowedMethod] = route.Handler
		return nil
	}

	if route.Path[0] == '/' {
		route.Path = route.Path[1:]
	} else {
		return ErrMalformedRoute
	}

	slashIndex := strings.Index(route.Path, "/")
	if slashIndex == 0 {
		// first character is a slash, that means the URL provided looks something like this:
		// /path/to//document # note the double slash
		return ErrMalformedRoute
	}

	var pathFragmentForChildNode string

	if slashIndex == -1 {
		pathFragmentForChildNode = route.Path
	} else {
		pathFragmentForChildNode = route.Path[0:slashIndex]
	}

	//Check that all characters in path are valid
	if !validCharacter(pathFragmentForChildNode) {
		return ErrInvalidCharacter
	}

	if strings.HasPrefix(pathFragmentForChildNode, "*") {
		wildChildRoute := Route{
			AllowedMethod: route.AllowedMethod,
			Path:          route.Path,
			Handler:       route.Handler,
		}
		return this.addWildcardChild(wildChildRoute, pathFragmentForChildNode)
	}

	if strings.HasPrefix(pathFragmentForChildNode, ":") {
		variableChildRoute := Route{
			AllowedMethod: route.AllowedMethod,
			Path:          route.Path,
			Handler:       route.Handler,
		}
		return this.addVariableChild(variableChildRoute, pathFragmentForChildNode)
	}

	staticChildRoute := Route{
		AllowedMethod: route.AllowedMethod,
		Path:          route.Path,
		Handler:       route.Handler,
	}
	return this.addStaticChild(staticChildRoute, pathFragmentForChildNode)
}
func (this *treeNode) addWildcardChild(route Route, pathFragment string) error {
	// validate incoming route.Path (must only be "*")
	if len(route.Path) > 1 {
		return ErrInvalidWildCard
	}
	route.Path = "" // now truncate it to ""

	if this.wildcardChild != nil {
		// wildcard child already exists, attach a handler for the specific method
		return this.wildcardChild.Add(route)
	}

	this.wildcardChild = &treeNode{pathFragment: pathFragment, handlers: map[Method]http.Handler{}}
	return this.wildcardChild.Add(route)
}
func (this *treeNode) addVariableChild(route Route, pathFragment string) error {
	route.Path = route.Path[len(pathFragment):]
	//TODO: create error checking function
	if this.variableChild != nil {
		return this.variableChild.Add(route)
	}

	this.variableChild = &treeNode{pathFragment: pathFragment, handlers: map[Method]http.Handler{}}
	return this.variableChild.Add(route)
}

func (this *treeNode) addStaticChild(route Route, pathFragment string) (err error) {
	route.Path = route.Path[len(pathFragment):]

	for _, staticChild := range this.staticChildren {
		if staticChild.pathFragment == pathFragment {
			return staticChild.Add(route)
		}
	}

	staticChild := &treeNode{pathFragment: pathFragment, handlers: map[Method]http.Handler{}}

	if err = staticChild.Add(route); err != nil {
		return err
	}

	this.staticChildren = append(this.staticChildren, staticChild)
	return nil
}

func (this *treeNode) Resolve(method Method, incomingPath string) (http.Handler, bool) {
	//TODO: return 405 error if the handler is nil
	if len(incomingPath) == 0 {
		return this.handlers[method], true // why true? because we got to a place where the resource exists
	}
	if incomingPath[0] == '/' {
		incomingPath = incomingPath[1:]
	}

	slashIndex := strings.Index(incomingPath, "/")

	var pathFragment string
	if slashIndex == -1 {
		pathFragment = incomingPath
	} else {
		pathFragment = incomingPath[0:slashIndex]
	}

	var resourceExists bool
	for _, staticChild := range this.staticChildren {
		if strings.Compare(pathFragment, staticChild.pathFragment) != 0 {
			continue // the child doesn't match, skip it
		}

		// at this point, the path fragment DOES match...
		remainingPath := incomingPath[len(staticChild.pathFragment):]
		handler, resourceExists := staticChild.Resolve(method, remainingPath)
		if handler != nil {
			return handler, resourceExists
		}

		break // don't bother checking any more of siblings of the static child, they don't match
	}

	if this.variableChild != nil {
		if strings.HasPrefix(incomingPath, this.variableChild.pathFragment) {
			remainingPath := incomingPath[len(this.variableChild.pathFragment):]
			handler, resourceExists := this.variableChild.Resolve(method, remainingPath)
			if handler != nil {
				return handler, resourceExists
			}
		}
	}

	if this.wildcardChild != nil {
		if strings.HasPrefix(incomingPath, this.wildcardChild.pathFragment) {
			return this.wildcardChild.Resolve(method, "") // wildcard matches everything, don't bother with the path
		}
	}

	//TODO: nothing matches -- return 404 error
	return nil, resourceExists // no wildcard children
}

func validCharacter(input string) bool {
	for index, r := range input {
		if unicode.IsLetter(r) {
			continue
		}
		if unicode.IsDigit(r) {
			continue
		}
		if isSpecialCharacter(r) {
			continue
		}
		if index == 0 && (r == '*' || r == ':') {
			continue
		}
		return false
	}
	return true
}
func isSpecialCharacter(r rune) bool {
	return r == '.' || r == '-' || r == '_'
}
